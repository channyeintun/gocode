package skills

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	pathpkg "path"
	"strings"
)

//go:embed builtins/*.md
var builtinSkillFiles embed.FS

func loadBuiltinSkills() ([]Skill, error) {
	entries, err := fs.ReadDir(builtinSkillFiles, "builtins")
	if err != nil {
		return nil, err
	}

	var skills []Skill
	var loadErrs []error
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := pathpkg.Join("builtins", entry.Name())
		data, err := builtinSkillFiles.ReadFile(path)
		if err != nil {
			loadErrs = append(loadErrs, fmt.Errorf("read %s: %w", path, err))
			continue
		}
		skills = append(skills, parseSkillContent(path, string(data)))
	}

	return skills, errors.Join(loadErrs...)
}
