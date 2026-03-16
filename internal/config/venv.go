package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// VenvInfo holds information about a detected Python virtual environment.
type VenvInfo struct {
	// Name is the display name (e.g., ".venv", "venv", "myenv")
	Name string
	// Path is the absolute path to the venv root directory
	Path string
	// BinDir is the path to the bin (or Scripts on Windows) directory
	BinDir string
}

// venvDirNames are the directory names to check for virtual environments.
var venvDirNames = []string{".venv", "venv", ".env", "env"}

// DetectVenv checks for Python virtual environments in the following order:
// 1. $VIRTUAL_ENV environment variable
// 2. Known venv directory names in the project root
// Returns nil if no venv is detected.
func DetectVenv(workDir string) *VenvInfo {
	// Check VIRTUAL_ENV env var first
	if venvPath := os.Getenv("VIRTUAL_ENV"); venvPath != "" {
		binDir := venvBinDir(venvPath)
		if _, err := os.Stat(binDir); err == nil {
			return &VenvInfo{
				Name:   filepath.Base(venvPath),
				Path:   venvPath,
				BinDir: binDir,
			}
		}
	}

	// Check known directory names in project root
	if workDir == "" {
		return nil
	}
	for _, name := range venvDirNames {
		venvPath := filepath.Join(workDir, name)
		binDir := venvBinDir(venvPath)
		if _, err := os.Stat(binDir); err == nil {
			// Verify it looks like a venv (has python binary)
			pythonPath := filepath.Join(binDir, "python")
			if runtime.GOOS == "windows" {
				pythonPath = filepath.Join(binDir, "python.exe")
			}
			if _, err := os.Stat(pythonPath); err == nil {
				return &VenvInfo{
					Name:   name,
					Path:   venvPath,
					BinDir: binDir,
				}
			}
		}
	}

	return nil
}

// DetectAllVenvs finds all virtual environments in the project directory.
// Returns a list of VenvInfo for each detected venv.
func DetectAllVenvs(workDir string) []*VenvInfo {
	if workDir == "" {
		return nil
	}

	var venvs []*VenvInfo
	seen := make(map[string]bool)

	// Check VIRTUAL_ENV env var
	if venvPath := os.Getenv("VIRTUAL_ENV"); venvPath != "" {
		binDir := venvBinDir(venvPath)
		if _, err := os.Stat(binDir); err == nil {
			venvs = append(venvs, &VenvInfo{
				Name:   filepath.Base(venvPath),
				Path:   venvPath,
				BinDir: binDir,
			})
			seen[venvPath] = true
		}
	}

	// Check known directory names
	for _, name := range venvDirNames {
		venvPath := filepath.Join(workDir, name)
		if seen[venvPath] {
			continue
		}
		binDir := venvBinDir(venvPath)
		if _, err := os.Stat(binDir); err == nil {
			pythonPath := filepath.Join(binDir, "python")
			if runtime.GOOS == "windows" {
				pythonPath = filepath.Join(binDir, "python.exe")
			}
			if _, err := os.Stat(pythonPath); err == nil {
				venvs = append(venvs, &VenvInfo{
					Name:   name,
					Path:   venvPath,
					BinDir: binDir,
				})
			}
		}
	}

	return venvs
}

// venvBinDir returns the bin directory for a venv path.
func venvBinDir(venvPath string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(venvPath, "Scripts")
	}
	return filepath.Join(venvPath, "bin")
}

// PrependVenvToPath creates a new PATH string with the venv bin directory prepended.
func PrependVenvToPath(venv *VenvInfo, currentPath string) string {
	if venv == nil || venv.BinDir == "" {
		return currentPath
	}
	return venv.BinDir + string(os.PathListSeparator) + currentPath
}

// VenvEnv returns environment variables with the venv PATH prepended.
// Returns nil if no venv is active (use default environment).
func VenvEnv(venv *VenvInfo) []string {
	if venv == nil {
		return nil
	}

	env := os.Environ()
	pathKey := "PATH="
	newPath := PrependVenvToPath(venv, os.Getenv("PATH"))

	var result []string
	pathSet := false
	for _, e := range env {
		if strings.HasPrefix(strings.ToUpper(e), "PATH=") {
			result = append(result, pathKey+newPath)
			pathSet = true
		} else {
			result = append(result, e)
		}
	}
	if !pathSet {
		result = append(result, pathKey+newPath)
	}

	// Also set VIRTUAL_ENV
	venvSet := false
	for i, e := range result {
		if strings.HasPrefix(e, "VIRTUAL_ENV=") {
			result[i] = "VIRTUAL_ENV=" + venv.Path
			venvSet = true
			break
		}
	}
	if !venvSet {
		result = append(result, "VIRTUAL_ENV="+venv.Path)
	}

	return result
}
