package lsp

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// ServerConfig holds the command to launch a language server
type ServerConfig struct {
	Command    string
	Args       []string
	LanguageID string
}

// ServerForFile returns the LSP server config for a file, or nil if none
func ServerForFile(filePath string) *ServerConfig {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		if _, err := exec.LookPath("gopls"); err == nil {
			return &ServerConfig{Command: "gopls", LanguageID: "go"}
		}
	case ".py":
		// Try pyright first, then pylsp
		if _, err := exec.LookPath("pyright-langserver"); err == nil {
			return &ServerConfig{Command: "pyright-langserver", Args: []string{"--stdio"}, LanguageID: "python"}
		}
		if _, err := exec.LookPath("pylsp"); err == nil {
			return &ServerConfig{Command: "pylsp", LanguageID: "python"}
		}
	case ".rs":
		if _, err := exec.LookPath("rust-analyzer"); err == nil {
			return &ServerConfig{Command: "rust-analyzer", LanguageID: "rust"}
		}
	case ".c", ".h":
		if _, err := exec.LookPath("clangd"); err == nil {
			return &ServerConfig{Command: "clangd", LanguageID: "c"}
		}
	case ".cpp", ".cc", ".cxx", ".hpp":
		if _, err := exec.LookPath("clangd"); err == nil {
			return &ServerConfig{Command: "clangd", LanguageID: "cpp"}
		}
	case ".js", ".jsx":
		if _, err := exec.LookPath("typescript-language-server"); err == nil {
			return &ServerConfig{Command: "typescript-language-server", Args: []string{"--stdio"}, LanguageID: "javascript"}
		}
	case ".ts", ".tsx":
		if _, err := exec.LookPath("typescript-language-server"); err == nil {
			return &ServerConfig{Command: "typescript-language-server", Args: []string{"--stdio"}, LanguageID: "typescript"}
		}
	case ".java":
		if _, err := exec.LookPath("jdtls"); err == nil {
			return &ServerConfig{Command: "jdtls", LanguageID: "java"}
		}
	}
	return nil
}

// LSPInstallCommands maps language IDs to their LSP server install commands.
var LSPInstallCommands = map[string]string{
	"go":         "go install golang.org/x/tools/gopls@latest",
	"python":     "pip install pyright",
	"rust":       "rustup component add rust-analyzer",
	"c":          "install clangd",
	"cpp":        "install clangd",
	"javascript": "npm install -g typescript-language-server typescript",
	"typescript": "npm install -g typescript-language-server typescript",
	"java":       "install jdtls",
}

// LanguageIDForFile returns the LSP languageId for a file extension
func LanguageIDForFile(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	default:
		return ""
	}
}
