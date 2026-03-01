package runner

import (
	"path/filepath"
	"strings"
)

// LangConfig holds build/run commands for a language
type LangConfig struct {
	Name       string
	Extensions []string
	BuildCmd   func(filePath string) (string, []string)
	RunCmd     func(filePath string) (string, []string)
}

var Languages = map[string]*LangConfig{
	"c": {
		Name:       "C",
		Extensions: []string{".c"},
		BuildCmd: func(filePath string) (string, []string) {
			out := strings.TrimSuffix(filePath, filepath.Ext(filePath))
			return "gcc", []string{filePath, "-o", out, "-Wall"}
		},
		RunCmd: func(filePath string) (string, []string) {
			out := strings.TrimSuffix(filePath, filepath.Ext(filePath))
			return out, nil
		},
	},
	"cpp": {
		Name:       "C++",
		Extensions: []string{".cpp", ".cc", ".cxx"},
		BuildCmd: func(filePath string) (string, []string) {
			out := strings.TrimSuffix(filePath, filepath.Ext(filePath))
			return "g++", []string{filePath, "-o", out, "-Wall", "-std=c++17"}
		},
		RunCmd: func(filePath string) (string, []string) {
			out := strings.TrimSuffix(filePath, filepath.Ext(filePath))
			return out, nil
		},
	},
	"python": {
		Name:       "Python",
		Extensions: []string{".py"},
		BuildCmd:   nil,
		RunCmd: func(filePath string) (string, []string) {
			return "python3", []string{filePath}
		},
	},
	"rust": {
		Name:       "Rust",
		Extensions: []string{".rs"},
		BuildCmd: func(filePath string) (string, []string) {
			out := strings.TrimSuffix(filePath, filepath.Ext(filePath))
			return "rustc", []string{filePath, "-o", out}
		},
		RunCmd: func(filePath string) (string, []string) {
			out := strings.TrimSuffix(filePath, filepath.Ext(filePath))
			return out, nil
		},
	},
	"go": {
		Name:       "Go",
		Extensions: []string{".go"},
		BuildCmd: func(filePath string) (string, []string) {
			return "go", []string{"build", filePath}
		},
		RunCmd: func(filePath string) (string, []string) {
			return "go", []string{"run", filePath}
		},
	},
	"javascript": {
		Name:       "JavaScript",
		Extensions: []string{".js", ".jsx"},
		BuildCmd:   nil,
		RunCmd: func(filePath string) (string, []string) {
			return "node", []string{filePath}
		},
	},
	"typescript": {
		Name:       "TypeScript",
		Extensions: []string{".ts", ".tsx"},
		BuildCmd:   nil,
		RunCmd: func(filePath string) (string, []string) {
			return "npx", []string{"tsx", filePath}
		},
	},
	"java": {
		Name:       "Java",
		Extensions: []string{".java"},
		BuildCmd: func(filePath string) (string, []string) {
			return "javac", []string{filePath}
		},
		RunCmd: func(filePath string) (string, []string) {
			dir := filepath.Dir(filePath)
			name := strings.TrimSuffix(filepath.Base(filePath), ".java")
			return "java", []string{"-cp", dir, name}
		},
	},
}

// DetectLanguage returns the language config for a file
func DetectLanguage(filePath string) *LangConfig {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, lang := range Languages {
		for _, e := range lang.Extensions {
			if e == ext {
				return lang
			}
		}
	}
	return nil
}
