package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const backgroundCommandMaxOutputBytes = 256 * 1024
const backgroundCommandSummaryPreviewBytes = 160
const backgroundCommandNotificationPreviewBytes = 4096

const backgroundCommandRetention = 5 * time.Minute

type backgroundCommand struct {
	mu                        sync.Mutex
	consumeMu                 sync.Mutex
	id                        string
	command                   string
	cwd                       string
	cmd                       *exec.Cmd
	stdin                     io.WriteCloser
	terminal                  *os.File
	cancel                    context.CancelFunc
	output                    *boundedOutput
	running                   bool
	exitCode                  *int
	errText                   string
	suppressAsyncNotification bool
	done                      chan struct{}
	startedAt                 time.Time
	updatedAt                 time.Time
}

type boundedOutput struct {
	mu               sync.Mutex
	data             []byte
	readOffset       int
	droppedUnreadLen int
}

type BackgroundCommandResult struct {
	CommandID string    `json:"CommandId"`
	Command   string    `json:"Command,omitempty"`
	Cwd       string    `json:"Cwd,omitempty"`
	Running   bool      `json:"Running"`
	StartedAt time.Time `json:"StartedAt,omitempty"`
	UpdatedAt time.Time `json:"UpdatedAt,omitempty"`
	Output    string    `json:"Output,omitempty"`
	Error     string    `json:"Error,omitempty"`
	ExitCode  *int      `json:"ExitCode,omitempty"`
}

type BackgroundCommandDetail struct {
	CommandID       string
	Command         string
	Cwd             string
	Status          string
	Running         bool
	StartedAt       time.Time
	UpdatedAt       time.Time
	Output          string
	HasUnreadOutput bool
	UnreadBytes     int
	ExitCode        *int
	Error           string
}

// BackgroundCommandUpdate is emitted when a retained background command changes
// state asynchronously outside the active tool turn.
type BackgroundCommandUpdate struct {
	CommandID       string
	Command         string
	Cwd             string
	Status          string
	Running         bool
	StartedAt       time.Time
	UpdatedAt       time.Time
	OutputPreview   string
	HasUnreadOutput bool
	UnreadBytes     int
	ExitCode        *int
	Error           string
}

var (
	backgroundCommands   = make(map[string]*backgroundCommand)
	backgroundCommandsMu sync.RWMutex
	backgroundCounter    uint64
	backgroundNotifierMu sync.RWMutex
	backgroundNotifier   func(BackgroundCommandUpdate)
)

// SetBackgroundCommandNotifier configures a process-local callback for
// asynchronous background command state updates.
func SetBackgroundCommandNotifier(fn func(BackgroundCommandUpdate)) {
	backgroundNotifierMu.Lock()
	defer backgroundNotifierMu.Unlock()
	backgroundNotifier = fn
}

func emitBackgroundCommandUpdate(update BackgroundCommandUpdate) {
	backgroundNotifierMu.RLock()
	fn := backgroundNotifier
	backgroundNotifierMu.RUnlock()
	if fn != nil {
		fn(update)
	}
}

func listBackgroundCommands(includeCompleted bool) []backgroundCommandSummary {
	backgroundCommandsMu.RLock()
	commands := make([]*backgroundCommand, 0, len(backgroundCommands))
	for _, bg := range backgroundCommands {
		commands = append(commands, bg)
	}
	backgroundCommandsMu.RUnlock()

	summaries := make([]backgroundCommandSummary, 0, len(commands))
	for _, bg := range commands {
		summary := bg.summary()
		if !includeCompleted && !summary.Running {
			continue
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func startBackgroundShellCommand(command, cwd string) (*backgroundCommand, error) {
	id := fmt.Sprintf("cmd_%d", atomic.AddUint64(&backgroundCounter, 1))
	cmd, err := shellCommand(command)
	if err != nil {
		return nil, err
	}
	cmd.Dir = cwd
	streamCtx, cancel := context.WithCancel(context.Background())
	if runtime.GOOS == "windows" {
		return startBackgroundPipeCommand(streamCtx, cancel, id, command, cwd, cmd)
	}
	return startBackgroundPTYCommand(streamCtx, cancel, id, command, cwd, cmd)
}

func startBackgroundPTYCommand(streamCtx context.Context, cancel context.CancelFunc, id, command, cwd string, cmd *exec.Cmd) (*backgroundCommand, error) {
	terminal, err := pty.Start(cmd)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("start background command in pty: %w", err)
	}

	bg := &backgroundCommand{
		id:        id,
		command:   command,
		cwd:       cwd,
		cmd:       cmd,
		stdin:     terminal,
		terminal:  terminal,
		cancel:    cancel,
		output:    &boundedOutput{},
		running:   true,
		done:      make(chan struct{}),
		startedAt: time.Now(),
		updatedAt: time.Now(),
	}

	backgroundCommandsMu.Lock()
	backgroundCommands[id] = bg
	backgroundCommandsMu.Unlock()

	go streamBackgroundOutput(streamCtx, bg, bg.output, terminal)
	go waitForBackgroundCommand(bg)

	return bg, nil
}

func startBackgroundPipeCommand(streamCtx context.Context, cancel context.CancelFunc, id, command, cwd string, cmd *exec.Cmd) (*backgroundCommand, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("open background command stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		_ = stdin.Close()
		return nil, fmt.Errorf("open background command stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, fmt.Errorf("open background command stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		_ = stdin.Close()
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, fmt.Errorf("start background command: %w", err)
	}

	bg := &backgroundCommand{
		id:        id,
		command:   command,
		cwd:       cwd,
		cmd:       cmd,
		stdin:     stdin,
		cancel:    cancel,
		output:    &boundedOutput{},
		running:   true,
		done:      make(chan struct{}),
		startedAt: time.Now(),
		updatedAt: time.Now(),
	}

	backgroundCommandsMu.Lock()
	backgroundCommands[id] = bg
	backgroundCommandsMu.Unlock()

	go streamBackgroundOutput(streamCtx, bg, bg.output, stdout)
	go streamBackgroundOutput(streamCtx, bg, bg.output, stderr)
	go waitForBackgroundCommand(bg)

	return bg, nil
}

func waitForBackgroundCommand(bg *backgroundCommand) {
	err := bg.cmd.Wait()

	bg.mu.Lock()
	if bg.cancel != nil {
		bg.cancel()
		bg.cancel = nil
	}
	if bg.terminal != nil {
		_ = bg.terminal.Close()
		bg.terminal = nil
		bg.stdin = nil
	} else if bg.stdin != nil {
		_ = bg.stdin.Close()
		bg.stdin = nil
	}

	bg.running = false
	bg.updatedAt = time.Now()
	notify := !bg.suppressAsyncNotification
	if err == nil {
		exitCode := 0
		bg.exitCode = &exitCode
		bg.mu.Unlock()
		close(bg.done)
		scheduleBackgroundCommandCleanup(bg)
		if notify {
			emitBackgroundCommandUpdate(bg.asyncUpdate())
		}
		return
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode := exitErr.ExitCode()
		bg.exitCode = &exitCode
		bg.errText = err.Error()
		bg.mu.Unlock()
		close(bg.done)
		scheduleBackgroundCommandCleanup(bg)
		if notify {
			emitBackgroundCommandUpdate(bg.asyncUpdate())
		}
		return
	}

	bg.errText = err.Error()
	bg.mu.Unlock()
	close(bg.done)
	scheduleBackgroundCommandCleanup(bg)
	if notify {
		emitBackgroundCommandUpdate(bg.asyncUpdate())
	}
}

func streamBackgroundOutput(ctx context.Context, bg *backgroundCommand, buffer *boundedOutput, reader io.ReadCloser) {
	chunk := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if deadlineReader, ok := reader.(interface{ SetReadDeadline(time.Time) error }); ok {
			_ = deadlineReader.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
		}
		readLen, err := reader.Read(chunk)
		if readLen > 0 {
			_, _ = buffer.Write(chunk[:readLen])
			bg.markUpdated(time.Now())
		}
		if err == nil {
			continue
		}
		if timeoutErr, ok := err.(interface{ Timeout() bool }); ok && timeoutErr.Timeout() {
			continue
		}
		if errors.Is(err, io.EOF) || errors.Is(err, syscall.EIO) {
			return
		}
		_, _ = buffer.Write([]byte(fmt.Sprintf("\n[Background PTY stream closed: %v]\n", err)))
		return
	}
}

func shutdownBackgroundCommands() {
	backgroundCommandsMu.RLock()
	commands := make([]*backgroundCommand, 0, len(backgroundCommands))
	for _, bg := range backgroundCommands {
		commands = append(commands, bg)
	}
	backgroundCommandsMu.RUnlock()

	for _, bg := range commands {
		bg.shutdown()
	}
}

// ShutdownBackgroundCommandsForSession terminates any still-running background
// commands so their PTY readers do not outlive engine shutdown.
func ShutdownBackgroundCommandsForSession() {
	shutdownBackgroundCommands()
}

func (bg *backgroundCommand) shutdown() {
	bg.mu.Lock()
	bg.suppressAsyncNotification = true
	if bg.cancel != nil {
		bg.cancel()
		bg.cancel = nil
	}
	terminal := bg.terminal
	bg.terminal = nil
	stdin := bg.stdin
	bg.stdin = nil
	cmd := bg.cmd
	running := bg.running
	bg.mu.Unlock()

	if terminal != nil {
		_ = terminal.Close()
	} else if stdin != nil {
		_ = stdin.Close()
	}
	if running && cmd != nil && cmd.Process != nil {
		_ = terminateBackgroundProcessTree(cmd)
	}
}

func terminateBackgroundProcessTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/f", "/t", "/pid", strconv.Itoa(cmd.Process.Pid)).Run()
	}
	return cmd.Process.Kill()
}

func scheduleBackgroundCommandCleanup(bg *backgroundCommand) {
	time.AfterFunc(backgroundCommandRetention, func() {
		backgroundCommandsMu.Lock()
		defer backgroundCommandsMu.Unlock()

		current, ok := backgroundCommands[bg.id]
		if !ok || current != bg {
			return
		}
		current.mu.Lock()
		defer current.mu.Unlock()
		if current.running {
			return
		}
		delete(backgroundCommands, bg.id)
	})
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

func forgetBackgroundCommand(commandID string) (BackgroundCommandResult, error) {
	backgroundCommandsMu.Lock()
	bg, ok := backgroundCommands[commandID]
	if !ok {
		backgroundCommandsMu.Unlock()
		return BackgroundCommandResult{}, fmt.Errorf("command %q not found", commandID)
	}

	bg.consumeMu.Lock()
	defer bg.consumeMu.Unlock()

	bg.mu.Lock()
	if bg.running {
		bg.mu.Unlock()
		backgroundCommandsMu.Unlock()
		return BackgroundCommandResult{}, fmt.Errorf("command %q is still running; stop it before forgetting it", bg.id)
	}

	result := BackgroundCommandResult{
		CommandID: bg.id,
		Command:   bg.command,
		Cwd:       bg.cwd,
		Running:   false,
		StartedAt: bg.startedAt,
		UpdatedAt: bg.updatedAt,
		Error:     bg.errText,
	}
	if bg.exitCode != nil {
		copied := *bg.exitCode
		result.ExitCode = &copied
	}
	bg.mu.Unlock()

	delete(backgroundCommands, commandID)
	backgroundCommandsMu.Unlock()

	result.Output = bg.output.ReadDelta()
	return result, nil
}

func (bg *backgroundCommand) sendInput(input string, wait time.Duration) (BackgroundCommandResult, error) {
	bg.consumeMu.Lock()
	defer bg.consumeMu.Unlock()

	bg.mu.Lock()
	if !bg.running {
		bg.mu.Unlock()
		return BackgroundCommandResult{}, fmt.Errorf("command %q is not running", bg.id)
	}
	_, err := io.WriteString(bg.stdin, input)
	if err == nil {
		bg.updatedAt = time.Now()
	}
	bg.mu.Unlock()
	if err != nil {
		return BackgroundCommandResult{}, fmt.Errorf("write command input: %w", err)
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

func (bg *backgroundCommand) status(wait time.Duration) BackgroundCommandResult {
	bg.consumeMu.Lock()
	defer bg.consumeMu.Unlock()

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

func (bg *backgroundCommand) stop(wait time.Duration) BackgroundCommandResult {
	bg.consumeMu.Lock()
	defer bg.consumeMu.Unlock()

	bg.shutdown()
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

func (bg *backgroundCommand) snapshotDelta() BackgroundCommandResult {
	bg.mu.Lock()
	running := bg.running
	errText := bg.errText
	var exitCode *int
	if bg.exitCode != nil {
		copied := *bg.exitCode
		exitCode = &copied
	}
	bg.mu.Unlock()

	return BackgroundCommandResult{
		CommandID: bg.id,
		Command:   bg.command,
		Cwd:       bg.cwd,
		Running:   running,
		StartedAt: bg.startedAt,
		UpdatedAt: bg.updatedAt,
		Output:    bg.output.ReadDelta(),
		Error:     errText,
		ExitCode:  exitCode,
	}
}

func (bg *backgroundCommand) detail(limit int) BackgroundCommandDetail {
	bg.mu.Lock()
	running := bg.running
	errText := bg.errText
	commandID := bg.id
	command := bg.command
	cwd := bg.cwd
	startedAt := bg.startedAt
	updatedAt := bg.updatedAt
	var exitCode *int
	if bg.exitCode != nil {
		copied := *bg.exitCode
		exitCode = &copied
	}
	bg.mu.Unlock()

	unread := bg.output.unreadSummary(limit)

	return BackgroundCommandDetail{
		CommandID:       commandID,
		Command:         command,
		Cwd:             cwd,
		Status:          backgroundCommandAsyncStatus(running, exitCode, errText),
		Running:         running,
		StartedAt:       startedAt,
		UpdatedAt:       updatedAt,
		Output:          bg.output.tail(limit),
		HasUnreadOutput: unread.HasUnread,
		UnreadBytes:     unread.UnreadBytes,
		ExitCode:        exitCode,
		Error:           errText,
	}
}

func (bg *backgroundCommand) summary() backgroundCommandSummary {
	bg.mu.Lock()
	startedAt := bg.startedAt
	updatedAt := bg.updatedAt
	defer bg.mu.Unlock()

	var exitCode *int
	if bg.exitCode != nil {
		copied := *bg.exitCode
		exitCode = &copied
	}
	unread := bg.output.unreadSummary(backgroundCommandSummaryPreviewBytes)

	return backgroundCommandSummary{
		CommandID:       bg.id,
		Command:         bg.command,
		Cwd:             bg.cwd,
		Running:         bg.running,
		Error:           bg.errText,
		ExitCode:        exitCode,
		StartedAt:       startedAt,
		UpdatedAt:       updatedAt,
		HasUnreadOutput: unread.HasUnread,
		UnreadBytes:     unread.UnreadBytes,
		UnreadPreview:   unread.Preview,
	}
}

func (bg *backgroundCommand) markUpdated(at time.Time) {
	bg.mu.Lock()
	defer bg.mu.Unlock()
	bg.updatedAt = at
}

func (bg *backgroundCommand) asyncUpdate() BackgroundCommandUpdate {
	bg.mu.Lock()
	running := bg.running
	errText := bg.errText
	startedAt := bg.startedAt
	updatedAt := bg.updatedAt
	command := bg.command
	cwd := bg.cwd
	commandID := bg.id
	var exitCode *int
	if bg.exitCode != nil {
		copied := *bg.exitCode
		exitCode = &copied
	}
	bg.mu.Unlock()

	unread := bg.output.unreadSummary(backgroundCommandNotificationPreviewBytes)

	return BackgroundCommandUpdate{
		CommandID:       commandID,
		Command:         command,
		Cwd:             cwd,
		Status:          backgroundCommandAsyncStatus(running, exitCode, errText),
		Running:         running,
		StartedAt:       startedAt,
		UpdatedAt:       updatedAt,
		OutputPreview:   unread.Preview,
		HasUnreadOutput: unread.HasUnread,
		UnreadBytes:     unread.UnreadBytes,
		ExitCode:        exitCode,
		Error:           errText,
	}
}

func backgroundCommandAsyncStatus(running bool, exitCode *int, errText string) string {
	if running {
		return "running"
	}
	if exitCode != nil && *exitCode != 0 {
		return "failed"
	}
	if strings.TrimSpace(errText) != "" {
		return "failed"
	}
	return "completed"
}

func renderBackgroundCommandResult(result BackgroundCommandResult) (string, error) {
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
		if b.readOffset < trim {
			b.droppedUnreadLen += trim - b.readOffset
		}
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

	hadDroppedOutput := b.droppedUnreadLen > 0
	droppedUnreadLen := b.droppedUnreadLen
	b.droppedUnreadLen = 0

	if b.readOffset >= len(b.data) {
		if hadDroppedOutput {
			return fmt.Sprintf("[Older buffered output was dropped before it could be read (%d bytes)]", droppedUnreadLen)
		}
		return ""
	}
	delta := bytes.TrimSpace(b.data[b.readOffset:])
	b.readOffset = len(b.data)
	if hadDroppedOutput {
		if len(delta) == 0 {
			return fmt.Sprintf("[Older buffered output was dropped before it could be read (%d bytes)]", droppedUnreadLen)
		}
		return fmt.Sprintf("[Older buffered output was dropped before it could be read (%d bytes)]\n%s", droppedUnreadLen, delta)
	}
	return string(delta)
}

type unreadOutputSummary struct {
	HasUnread   bool
	UnreadBytes int
	Preview     string
}

func (b *boundedOutput) unreadSummary(limit int) unreadOutputSummary {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.readOffset >= len(b.data) {
		return unreadOutputSummary{}
	}

	unread := bytes.TrimSpace(b.data[b.readOffset:])
	if len(unread) == 0 {
		return unreadOutputSummary{}
	}

	previewBytes := unread
	truncated := false
	if limit > 0 && len(previewBytes) > limit {
		previewBytes = previewBytes[len(previewBytes)-limit:]
		truncated = true
	}
	preview := string(previewBytes)
	if truncated {
		preview = "[...]" + preview
	}
	if b.droppedUnreadLen > 0 {
		preview = fmt.Sprintf("[older unread output dropped: %d bytes]\n%s", b.droppedUnreadLen, preview)
	}

	return unreadOutputSummary{
		HasUnread:   true,
		UnreadBytes: len(unread),
		Preview:     preview,
	}
}

func (b *boundedOutput) tail(limit int) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.data) == 0 {
		return ""
	}

	tail := bytes.TrimSpace(b.data)
	if len(tail) == 0 {
		return ""
	}

	truncated := false
	if limit > 0 && len(tail) > limit {
		tail = tail[len(tail)-limit:]
		truncated = true
	}

	text := string(tail)
	if truncated {
		return "[...]" + text
	}
	return text
}

func InspectBackgroundCommand(ctx context.Context, commandID string, wait time.Duration, tailBytes int) (BackgroundCommandDetail, error) {
	bg, err := getBackgroundCommand(commandID)
	if err != nil {
		return BackgroundCommandDetail{}, err
	}

	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-bg.done:
		case <-timer.C:
		case <-ctx.Done():
			return BackgroundCommandDetail{}, ctx.Err()
		}
	}

	return bg.detail(tailBytes), nil
}

func StopBackgroundCommand(commandID string, wait time.Duration) (BackgroundCommandResult, error) {
	bg, err := getBackgroundCommand(commandID)
	if err != nil {
		return BackgroundCommandResult{}, err
	}
	return bg.stop(wait), nil
}

func BackgroundCommandUpdateSnapshot(commandID string) (BackgroundCommandUpdate, error) {
	bg, err := getBackgroundCommand(commandID)
	if err != nil {
		return BackgroundCommandUpdate{}, err
	}
	return bg.asyncUpdate(), nil
}
