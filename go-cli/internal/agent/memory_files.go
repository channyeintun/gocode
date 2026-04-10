package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// MemoryFile represents a loaded project instruction file (CLAUDE.md, rules, etc.).
type MemoryFile struct {
	Path    string
	Type    string // "user", "project", "local"
	Content string
}

const (
	memoryTypeUser    = "user"
	memoryTypeProject = "project"
	memoryTypeLocal   = "local"

	maxMemoryFileBytes = 40_000
	maxMemoryFiles     = 20
)

// LoadMemoryFiles discovers and loads instruction files in priority order:
//  1. User memory: ~/.claude/CLAUDE.md
//  2. Project memory: CLAUDE.md, .claude/CLAUDE.md, .claude/rules/*.md (walking up from cwd to root)
//  3. Local memory: CLAUDE.local.md (walking up from cwd to root)
//
// Files closer to the working directory have higher priority and are loaded later.
func LoadMemoryFiles() []MemoryFile {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	var files []MemoryFile

	// 1. User memory (~/.claude/CLAUDE.md)
	if home, err := os.UserHomeDir(); err == nil {
		userFile := filepath.Join(home, ".claude", "CLAUDE.md")
		if content, err := readMemoryFile(userFile); err == nil {
			files = append(files, MemoryFile{Path: userFile, Type: memoryTypeUser, Content: content})
		}
	}

	// 2+3. Walk from cwd up to root, collect project + local files
	// We walk upward and prepend so that files closer to cwd come last (higher priority).
	dirs := walkUpDirs(cwd)

	// Reverse so outermost directories load first (lowest priority).
	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		files = appendProjectFiles(files, dir)
	}

	// Trim to max files
	if len(files) > maxMemoryFiles {
		files = files[len(files)-maxMemoryFiles:]
	}

	return files
}

func appendProjectFiles(files []MemoryFile, dir string) []MemoryFile {
	// CLAUDE.md at directory root
	if content, err := readMemoryFile(filepath.Join(dir, "CLAUDE.md")); err == nil {
		files = append(files, MemoryFile{Path: filepath.Join(dir, "CLAUDE.md"), Type: memoryTypeProject, Content: content})
	}

	// .claude/CLAUDE.md
	if content, err := readMemoryFile(filepath.Join(dir, ".claude", "CLAUDE.md")); err == nil {
		files = append(files, MemoryFile{Path: filepath.Join(dir, ".claude", "CLAUDE.md"), Type: memoryTypeProject, Content: content})
	}

	// .claude/rules/*.md
	rulesDir := filepath.Join(dir, ".claude", "rules")
	if entries, err := os.ReadDir(rulesDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				continue
			}
			rulePath := filepath.Join(rulesDir, entry.Name())
			if content, err := readMemoryFile(rulePath); err == nil {
				files = append(files, MemoryFile{Path: rulePath, Type: memoryTypeProject, Content: content})
			}
		}
	}

	// CLAUDE.local.md (not checked into version control)
	if content, err := readMemoryFile(filepath.Join(dir, "CLAUDE.local.md")); err == nil {
		files = append(files, MemoryFile{Path: filepath.Join(dir, "CLAUDE.local.md"), Type: memoryTypeLocal, Content: content})
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

// FormatMemoryPrompt renders loaded memory files into a system prompt section.
func FormatMemoryPrompt(files []MemoryFile) string {
	if len(files) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("Codebase and user instructions are shown below. Be sure to adhere to these instructions. IMPORTANT: These instructions OVERRIDE any default behavior and you MUST follow them exactly as written.\n\n")

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
