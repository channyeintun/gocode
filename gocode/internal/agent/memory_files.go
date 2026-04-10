package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// MemoryFile represents a loaded instruction file.
type MemoryFile struct {
	Path    string
	Type    string // "project" or "local"
	Content string
}

const (
	memoryTypeProject = "project"
	memoryTypeLocal   = "local"

	maxMemoryFileBytes = 40_000
	maxMemoryFiles     = 20
)

// LoadMemoryFiles discovers and loads shared instruction files in priority order:
//  1. Project instructions: AGENTS.md (walking up from cwd to root)
//  2. Local instructions: AGENTS.local.md (walking up from cwd to root)
//
// Files closer to the working directory have higher priority and are loaded later.
func LoadMemoryFiles() []MemoryFile {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	var files []MemoryFile
	dirs := walkUpDirs(cwd)

	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		files = appendProjectFiles(files, dir)
	}

	if len(files) > maxMemoryFiles {
		files = files[len(files)-maxMemoryFiles:]
	}

	return files
}

func appendProjectFiles(files []MemoryFile, dir string) []MemoryFile {
	if content, err := readMemoryFile(filepath.Join(dir, "AGENTS.md")); err == nil {
		files = append(files, MemoryFile{Path: filepath.Join(dir, "AGENTS.md"), Type: memoryTypeProject, Content: content})
	}

	if content, err := readMemoryFile(filepath.Join(dir, "AGENTS.local.md")); err == nil {
		files = append(files, MemoryFile{Path: filepath.Join(dir, "AGENTS.local.md"), Type: memoryTypeLocal, Content: content})
	}

	return files
}

func walkUpDirs(start string) []string {
	var dirs []string
	dir := filepath.Clean(start)
	for {
		dirs = append(dirs, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

func readMemoryFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", os.ErrNotExist
	}
	if len(content) > maxMemoryFileBytes {
		content = content[:maxMemoryFileBytes] + "\n[truncated]"
	}
	return content, nil
}

// FormatMemoryPrompt renders loaded instruction files into a system prompt section.
func FormatMemoryPrompt(files []MemoryFile) string {
	if len(files) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Project instructions are shown below. Be sure to adhere to these instructions. IMPORTANT: These instructions override default behavior and should be followed exactly when applicable.\n\n")

	for _, f := range files {
		b.WriteString("<memory_file path=\"")
		b.WriteString(f.Path)
		b.WriteString("\" type=\"")
		b.WriteString(f.Type)
		b.WriteString("\">\n")
		b.WriteString(f.Content)
		b.WriteString("\n</memory_file>\n\n")
	}

	return strings.TrimSpace(b.String())
}
