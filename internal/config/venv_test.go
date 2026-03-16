package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// === Epic 25: Python Virtual Environment Support ===

func TestDetectVenv_NoVenv(t *testing.T) {
	tmpDir := t.TempDir()
	venv := DetectVenv(tmpDir)
	if venv != nil {
		t.Errorf("expected nil venv in empty directory, got %+v", venv)
	}
}

func TestDetectVenv_DotVenvDir(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := ".venv/bin"
	if runtime.GOOS == "windows" {
		binDir = ".venv/Scripts"
	}
	venvBin := filepath.Join(tmpDir, binDir)
	if err := os.MkdirAll(venvBin, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a python binary
	pythonName := "python"
	if runtime.GOOS == "windows" {
		pythonName = "python.exe"
	}
	pythonPath := filepath.Join(venvBin, pythonName)
	if err := os.WriteFile(pythonPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	venv := DetectVenv(tmpDir)
	if venv == nil {
		t.Fatal("expected to detect .venv directory")
	}
	if venv.Name != ".venv" {
		t.Errorf("expected name '.venv', got %q", venv.Name)
	}
	if venv.Path != filepath.Join(tmpDir, ".venv") {
		t.Errorf("expected path %q, got %q", filepath.Join(tmpDir, ".venv"), venv.Path)
	}
	if venv.BinDir != venvBin {
		t.Errorf("expected bin dir %q, got %q", venvBin, venv.BinDir)
	}
}

func TestDetectVenv_VenvDir(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := "venv/bin"
	if runtime.GOOS == "windows" {
		binDir = "venv/Scripts"
	}
	venvBin := filepath.Join(tmpDir, binDir)
	if err := os.MkdirAll(venvBin, 0755); err != nil {
		t.Fatal(err)
	}

	pythonName := "python"
	if runtime.GOOS == "windows" {
		pythonName = "python.exe"
	}
	if err := os.WriteFile(filepath.Join(venvBin, pythonName), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	venv := DetectVenv(tmpDir)
	if venv == nil {
		t.Fatal("expected to detect venv directory")
	}
	if venv.Name != "venv" {
		t.Errorf("expected name 'venv', got %q", venv.Name)
	}
}

func TestDetectVenv_PrefersEnvVar(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .venv directory
	binDir := ".venv/bin"
	if runtime.GOOS == "windows" {
		binDir = ".venv/Scripts"
	}
	venvBin := filepath.Join(tmpDir, binDir)
	if err := os.MkdirAll(venvBin, 0755); err != nil {
		t.Fatal(err)
	}
	pythonName := "python"
	if runtime.GOOS == "windows" {
		pythonName = "python.exe"
	}
	if err := os.WriteFile(filepath.Join(venvBin, pythonName), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create a separate venv and set VIRTUAL_ENV
	otherVenv := filepath.Join(tmpDir, "other-venv")
	otherBin := filepath.Join(otherVenv, "bin")
	if runtime.GOOS == "windows" {
		otherBin = filepath.Join(otherVenv, "Scripts")
	}
	if err := os.MkdirAll(otherBin, 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("VIRTUAL_ENV", otherVenv)

	venv := DetectVenv(tmpDir)
	if venv == nil {
		t.Fatal("expected to detect venv from VIRTUAL_ENV")
	}
	if venv.Name != "other-venv" {
		t.Errorf("expected name 'other-venv' from VIRTUAL_ENV, got %q", venv.Name)
	}
}

func TestDetectVenv_EmptyWorkDir(t *testing.T) {
	venv := DetectVenv("")
	if venv != nil {
		t.Error("expected nil venv for empty workDir")
	}
}

func TestDetectAllVenvs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both .venv and venv directories
	for _, name := range []string{".venv", "venv"} {
		binDir := filepath.Join(tmpDir, name, "bin")
		if runtime.GOOS == "windows" {
			binDir = filepath.Join(tmpDir, name, "Scripts")
		}
		if err := os.MkdirAll(binDir, 0755); err != nil {
			t.Fatal(err)
		}
		pythonName := "python"
		if runtime.GOOS == "windows" {
			pythonName = "python.exe"
		}
		if err := os.WriteFile(filepath.Join(binDir, pythonName), []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Clear VIRTUAL_ENV to avoid interference
	t.Setenv("VIRTUAL_ENV", "")

	venvs := DetectAllVenvs(tmpDir)
	if len(venvs) != 2 {
		t.Fatalf("expected 2 venvs, got %d", len(venvs))
	}
}

func TestPrependVenvToPath(t *testing.T) {
	venv := &VenvInfo{
		Name:   ".venv",
		Path:   "/project/.venv",
		BinDir: "/project/.venv/bin",
	}
	newPath := PrependVenvToPath(venv, "/usr/local/bin:/usr/bin")
	if !strings.HasPrefix(newPath, "/project/.venv/bin") {
		t.Errorf("expected venv bin at start of PATH, got %q", newPath)
	}
	if !strings.Contains(newPath, "/usr/local/bin") {
		t.Error("expected original PATH to be preserved")
	}
}

func TestPrependVenvToPath_NilVenv(t *testing.T) {
	originalPath := "/usr/local/bin:/usr/bin"
	newPath := PrependVenvToPath(nil, originalPath)
	if newPath != originalPath {
		t.Errorf("expected unchanged PATH with nil venv, got %q", newPath)
	}
}

func TestVenvEnv_NilVenv(t *testing.T) {
	env := VenvEnv(nil)
	if env != nil {
		t.Error("expected nil env for nil venv")
	}
}

func TestVenvEnv_SetsPathAndVirtualEnv(t *testing.T) {
	venv := &VenvInfo{
		Name:   ".venv",
		Path:   "/project/.venv",
		BinDir: "/project/.venv/bin",
	}

	env := VenvEnv(venv)
	if env == nil {
		t.Fatal("expected non-nil env")
	}

	pathFound := false
	venvFound := false
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			pathFound = true
			pathVal := strings.TrimPrefix(e, "PATH=")
			if !strings.HasPrefix(pathVal, "/project/.venv/bin") {
				t.Errorf("expected PATH to start with venv bin, got %q", pathVal)
			}
		}
		if e == "VIRTUAL_ENV=/project/.venv" {
			venvFound = true
		}
	}
	if !pathFound {
		t.Error("expected PATH in env")
	}
	if !venvFound {
		t.Error("expected VIRTUAL_ENV in env")
	}
}

func TestVenvBinDir(t *testing.T) {
	dir := venvBinDir("/project/.venv")
	if runtime.GOOS == "windows" {
		if dir != filepath.Join("/project/.venv", "Scripts") {
			t.Errorf("expected Scripts dir on Windows, got %q", dir)
		}
	} else {
		if dir != "/project/.venv/bin" {
			t.Errorf("expected bin dir on Unix, got %q", dir)
		}
	}
}
