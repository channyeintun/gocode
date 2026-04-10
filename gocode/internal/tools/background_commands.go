package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

const backgroundCommandMaxOutputBytes = 256 * 1024

type backgroundCommand struct {
	mu       sync.Mutex
	id       string
	command  string
	cwd      string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	output   *boundedOutput
	running  bool
	exitCode *int
	errText  string
	done     chan struct{}
}

type boundedOutput struct {
	mu         sync.Mutex
	data       []byte
	readOffset int
}

type backgroundCommandResult struct {
	CommandID string `json:"CommandId"`
	Running   bool   `json:"Running"`
	Output    string `json:"Output,omitempty"`
	Error     string `json:"Error,omitempty"`
	ExitCode  *int   `json:"ExitCode,omitempty"`
}

var (
	backgroundCommands   = make(map[string]*backgroundCommand)
	backgroundCommandsMu sync.RWMutex
	backgroundCounter    uint64
)

func startBackgroundShellCommand(command, cwd string) (*backgroundCommand, error) {
	id := fmt.Sprintf("cmd_%d", atomic.AddUint64(&backgroundCounter, 1))
	cmd := exec.Command("/bin/zsh", "-lc", command)
	cmd.Dir = cwd

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("open stderr pipe: %w", err)
	}

	bg := &backgroundCommand{
		id:      id,
		command: command,
		cwd:     cwd,
		cmd:     cmd,
		stdin:   stdin,
		output:  &boundedOutput{},
		running: true,
		done:    make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start background command: %w", err)
	}

	backgroundCommandsMu.Lock()
	backgroundCommands[id] = bg
	backgroundCommandsMu.Unlock()

	go streamBackgroundOutput(bg.output, stdout)
	go streamBackgroundOutput(bg.output, stderr)
	go waitForBackgroundCommand(bg)

	return bg, nil
}

func waitForBackgroundCommand(bg *backgroundCommand) {
	err := bg.cmd.Wait()

	bg.mu.Lock()
	defer bg.mu.Unlock()
	defer close(bg.done)

	bg.running = false
	if err == nil {
		exitCode := 0
		bg.exitCode = &exitCode
		return
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		bg.exitCode = &exitCode
		bg.errText = err.Error()
		return
	}

	bg.errText = err.Error()
}

func streamBackgroundOutput(buffer *boundedOutput, reader io.Reader) {
	_, _ = io.Copy(buffer, reader)
}

func getBackgroundCommand(commandID string) (*backgroundCommand, error) {
	backgroundCommandsMu.RLock()
	defer backgroundCommandsMu.RUnlock()

	bg, ok := backgroundCommands[commandID]
	if !ok {
		return nil, fmt.Errorf("command %q not found", commandID)
	}
	return bg, nil
}

func (bg *backgroundCommand) sendInput(input string, wait time.Duration) (backgroundCommandResult, error) {
	bg.mu.Lock()
	if !bg.running {
		bg.mu.Unlock()
		return backgroundCommandResult{}, fmt.Errorf("command %q is not running", bg.id)
	}
	_, err := io.WriteString(bg.stdin, input)
	bg.mu.Unlock()
	if err != nil {
		return backgroundCommandResult{}, fmt.Errorf("write command input: %w", err)
	}

	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-bg.done:
		case <-timer.C:
		}
	}

	return bg.snapshotDelta(), nil
}

func (bg *backgroundCommand) status(wait time.Duration) backgroundCommandResult {
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-bg.done:
		case <-timer.C:
		}
	}
	return bg.snapshotDelta()
}

func (bg *backgroundCommand) snapshotDelta() backgroundCommandResult {
	bg.mu.Lock()
	running := bg.running
	errText := bg.errText
	var exitCode *int
	if bg.exitCode != nil {
		copied := *bg.exitCode
		exitCode = &copied
	}
	bg.mu.Unlock()

	return backgroundCommandResult{
		CommandID: bg.id,
		Running:   running,
		Output:    bg.output.ReadDelta(),
		Error:     errText,
		ExitCode:  exitCode,
	}
}

func renderBackgroundCommandResult(result backgroundCommandResult) (string, error) {
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.data = append(b.data, p...)
	if len(b.data) > backgroundCommandMaxOutputBytes {
		trim := len(b.data) - backgroundCommandMaxOutputBytes
		b.data = append([]byte(nil), b.data[trim:]...)
		if b.readOffset > trim {
			b.readOffset -= trim
		} else {
			b.readOffset = 0
		}
	}
	return len(p), nil
}

func (b *boundedOutput) ReadDelta() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.readOffset >= len(b.data) {
		return ""
	}
	delta := bytes.TrimSpace(b.data[b.readOffset:])
	b.readOffset = len(b.data)
	return string(delta)
}
