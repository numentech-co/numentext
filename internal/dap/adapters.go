package dap

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// AdapterConfig holds config for launching a debug adapter
type AdapterConfig struct {
	Command    string
	Args       []string
	LanguageID string
}

// AdapterForFile returns the debug adapter config for a file, or nil
func AdapterForFile(filePath string) *AdapterConfig {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		if _, err := exec.LookPath("dlv"); err == nil {
			return &AdapterConfig{
				Command:    "dlv",
				Args:       []string{"dap"},
				LanguageID: "go",
			}
		}
	case ".py":
		if _, err := exec.LookPath("python3"); err == nil {
			return &AdapterConfig{
				Command:    "python3",
				Args:       []string{"-m", "debugpy.adapter"},
				LanguageID: "python",
			}
		}
	case ".rs":
		// lldb-vscode or codelldb
		if _, err := exec.LookPath("lldb-vscode"); err == nil {
			return &AdapterConfig{
				Command:    "lldb-vscode",
				LanguageID: "rust",
			}
		}
	case ".c", ".cpp", ".cc", ".cxx":
		if _, err := exec.LookPath("lldb-vscode"); err == nil {
			return &AdapterConfig{
				Command:    "lldb-vscode",
				LanguageID: "c",
			}
		}
	case ".java":
		if _, err := exec.LookPath("java"); err == nil {
			return &AdapterConfig{
				Command:    "java",
				Args:       []string{"-agentlib:jdwp=transport=dt_socket,server=y,suspend=y,address=5005"},
				LanguageID: "java",
			}
		}
	case ".kt", ".kts":
		// Kotlin uses the same JVM debugger
		if _, err := exec.LookPath("java"); err == nil {
			return &AdapterConfig{
				Command:    "java",
				Args:       []string{"-agentlib:jdwp=transport=dt_socket,server=y,suspend=y,address=5005"},
				LanguageID: "kotlin",
			}
		}
	}
	return nil
}
