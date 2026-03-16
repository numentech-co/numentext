package runner

import (
	"os"
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
			dir := filepath.Dir(filePath)
			// Maven project
			if fileExistsAt(filepath.Join(dir, "pom.xml")) {
				return "mvn", []string{"-f", filepath.Join(dir, "pom.xml"), "compile"}
			}
			// Gradle project (Kotlin DSL or Groovy DSL)
			if fileExistsAt(filepath.Join(dir, "build.gradle.kts")) || fileExistsAt(filepath.Join(dir, "build.gradle")) {
				return "gradle", []string{"-p", dir, "build"}
			}
			// Single file fallback
			return "javac", []string{filePath}
		},
		RunCmd: func(filePath string) (string, []string) {
			dir := filepath.Dir(filePath)
			// Maven project
			if fileExistsAt(filepath.Join(dir, "pom.xml")) {
				return "mvn", []string{"-f", filepath.Join(dir, "pom.xml"), "exec:java"}
			}
			// Gradle project
			if fileExistsAt(filepath.Join(dir, "build.gradle.kts")) || fileExistsAt(filepath.Join(dir, "build.gradle")) {
				return "gradle", []string{"-p", dir, "run"}
			}
			// Single file fallback
			name := strings.TrimSuffix(filepath.Base(filePath), ".java")
			return "java", []string{"-cp", dir, name}
		},
	},
	"kotlin": {
		Name:       "Kotlin",
		Extensions: []string{".kt", ".kts"},
		BuildCmd: func(filePath string) (string, []string) {
			dir := filepath.Dir(filePath)
			// Gradle project (Kotlin DSL or Groovy DSL)
			if fileExistsAt(filepath.Join(dir, "build.gradle.kts")) || fileExistsAt(filepath.Join(dir, "build.gradle")) {
				return "gradle", []string{"-p", dir, "build"}
			}
			// Maven project
			if fileExistsAt(filepath.Join(dir, "pom.xml")) {
				return "mvn", []string{"-f", filepath.Join(dir, "pom.xml"), "compile"}
			}
			// Single file: kotlinc compiles to jar
			base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
			return "kotlinc", []string{filePath, "-include-runtime", "-d", base + ".jar"}
		},
		RunCmd: func(filePath string) (string, []string) {
			dir := filepath.Dir(filePath)
			// Gradle project
			if fileExistsAt(filepath.Join(dir, "build.gradle.kts")) || fileExistsAt(filepath.Join(dir, "build.gradle")) {
				return "gradle", []string{"-p", dir, "run"}
			}
			// Maven project
			if fileExistsAt(filepath.Join(dir, "pom.xml")) {
				return "mvn", []string{"-f", filepath.Join(dir, "pom.xml"), "exec:java"}
			}
			// Single file: run the compiled jar
			base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
			return "java", []string{"-jar", base + ".jar"}
		},
	},
}

// fileExistsAt checks if a file exists at the given path.
func fileExistsAt(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
