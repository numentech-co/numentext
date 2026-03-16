package runner

import (
	"testing"
)

// === Story 5.1: Language Build Commands ===

func TestDetectLanguage_C(t *testing.T) {
	lang := DetectLanguage("main.c")
	if lang == nil || lang.Name != "C" {
		t.Errorf("expected C, got %v", lang)
	}
}

func TestDetectLanguage_Cpp(t *testing.T) {
	lang := DetectLanguage("app.cpp")
	if lang == nil || lang.Name != "C++" {
		t.Errorf("expected C++, got %v", lang)
	}
}

func TestDetectLanguage_Rust(t *testing.T) {
	lang := DetectLanguage("main.rs")
	if lang == nil || lang.Name != "Rust" {
		t.Errorf("expected Rust, got %v", lang)
	}
}

func TestDetectLanguage_Go(t *testing.T) {
	lang := DetectLanguage("main.go")
	if lang == nil || lang.Name != "Go" {
		t.Errorf("expected Go, got %v", lang)
	}
}

func TestDetectLanguage_Python(t *testing.T) {
	lang := DetectLanguage("script.py")
	if lang == nil || lang.Name != "Python" {
		t.Errorf("expected Python, got %v", lang)
	}
}

func TestDetectLanguage_JavaScript(t *testing.T) {
	lang := DetectLanguage("app.js")
	if lang == nil || lang.Name != "JavaScript" {
		t.Errorf("expected JavaScript, got %v", lang)
	}
}

func TestDetectLanguage_TypeScript(t *testing.T) {
	lang := DetectLanguage("app.ts")
	if lang == nil || lang.Name != "TypeScript" {
		t.Errorf("expected TypeScript, got %v", lang)
	}
}

func TestDetectLanguage_Java(t *testing.T) {
	lang := DetectLanguage("Main.java")
	if lang == nil || lang.Name != "Java" {
		t.Errorf("expected Java, got %v", lang)
	}
}

func TestDetectLanguage_Kotlin(t *testing.T) {
	lang := DetectLanguage("Main.kt")
	if lang == nil || lang.Name != "Kotlin" {
		t.Errorf("expected Kotlin, got %v", lang)
	}
}

func TestDetectLanguage_KotlinScript(t *testing.T) {
	lang := DetectLanguage("build.gradle.kts")
	if lang == nil || lang.Name != "Kotlin" {
		t.Errorf("expected Kotlin, got %v", lang)
	}
}

func TestDetectLanguage_Unknown(t *testing.T) {
	lang := DetectLanguage("readme.txt")
	if lang != nil {
		t.Errorf("expected nil for unknown extension, got %v", lang.Name)
	}
}

func TestCBuildCommand(t *testing.T) {
	lang := DetectLanguage("main.c")
	if lang.BuildCmd == nil {
		t.Fatal("C should have a build command")
	}
	cmd, args := lang.BuildCmd("/tmp/main.c")
	if cmd != "gcc" {
		t.Errorf("expected gcc, got %s", cmd)
	}
	found := false
	for _, a := range args {
		if a == "-Wall" {
			found = true
		}
	}
	if !found {
		t.Error("expected -Wall flag for C build")
	}
}

func TestCppBuildCommand(t *testing.T) {
	lang := DetectLanguage("app.cpp")
	cmd, args := lang.BuildCmd("/tmp/app.cpp")
	if cmd != "g++" {
		t.Errorf("expected g++, got %s", cmd)
	}
	hasWall, hasStd := false, false
	for _, a := range args {
		if a == "-Wall" {
			hasWall = true
		}
		if a == "-std=c++17" {
			hasStd = true
		}
	}
	if !hasWall || !hasStd {
		t.Error("expected -Wall and -std=c++17 flags for C++ build")
	}
}

func TestPythonNoBuildStep(t *testing.T) {
	lang := DetectLanguage("script.py")
	if lang.BuildCmd != nil {
		t.Error("Python should not have a build command")
	}
}

func TestJavaScriptNoBuildStep(t *testing.T) {
	lang := DetectLanguage("app.js")
	if lang.BuildCmd != nil {
		t.Error("JavaScript should not have a build command")
	}
}

func TestPythonRunCommand(t *testing.T) {
	lang := DetectLanguage("script.py")
	cmd, args := lang.RunCmd("/tmp/script.py")
	if cmd != "python3" {
		t.Errorf("expected python3, got %s", cmd)
	}
	if len(args) != 1 || args[0] != "/tmp/script.py" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestGoRunCommand(t *testing.T) {
	lang := DetectLanguage("main.go")
	cmd, args := lang.RunCmd("/tmp/main.go")
	if cmd != "go" {
		t.Errorf("expected go, got %s", cmd)
	}
	if len(args) < 2 || args[0] != "run" {
		t.Errorf("expected 'go run', got %v", args)
	}
}

func TestNodeRunCommand(t *testing.T) {
	lang := DetectLanguage("app.js")
	cmd, args := lang.RunCmd("/tmp/app.js")
	if cmd != "node" {
		t.Errorf("expected node, got %s", cmd)
	}
	if len(args) != 1 || args[0] != "/tmp/app.js" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestTypeScriptRunCommand(t *testing.T) {
	lang := DetectLanguage("app.ts")
	cmd, args := lang.RunCmd("/tmp/app.ts")
	if cmd != "npx" {
		t.Errorf("expected npx, got %s", cmd)
	}
	if len(args) < 2 || args[0] != "tsx" {
		t.Errorf("expected 'npx tsx', got %v", args)
	}
}

func TestJavaBuildAndRun(t *testing.T) {
	lang := DetectLanguage("Main.java")
	cmd, args := lang.BuildCmd("/tmp/src/Main.java")
	if cmd != "javac" {
		t.Errorf("expected javac, got %s", cmd)
	}
	if len(args) != 1 || args[0] != "/tmp/src/Main.java" {
		t.Errorf("unexpected build args: %v", args)
	}

	cmd, args = lang.RunCmd("/tmp/src/Main.java")
	if cmd != "java" {
		t.Errorf("expected java, got %s", cmd)
	}
	// Should have -cp /tmp/src Main
	if len(args) < 3 || args[0] != "-cp" {
		t.Errorf("unexpected run args: %v", args)
	}
}

func TestKotlinBuildAndRun(t *testing.T) {
	lang := DetectLanguage("Main.kt")
	if lang == nil {
		t.Fatal("expected Kotlin language config")
	}
	cmd, args := lang.BuildCmd("/tmp/Main.kt")
	if cmd != "kotlinc" {
		t.Errorf("expected kotlinc, got %s", cmd)
	}
	// Should have -include-runtime -d Main.jar
	foundRuntime := false
	for _, a := range args {
		if a == "-include-runtime" {
			foundRuntime = true
		}
	}
	if !foundRuntime {
		t.Error("expected -include-runtime flag for Kotlin build")
	}

	cmd, args = lang.RunCmd("/tmp/Main.kt")
	if cmd != "java" {
		t.Errorf("expected java, got %s", cmd)
	}
	if len(args) < 2 || args[0] != "-jar" {
		t.Errorf("expected '-jar Main.jar', got %v", args)
	}
}

// === Story 5.2: Run Execution ===

func TestFormatBuildCommand(t *testing.T) {
	cmd := FormatBuildCommand("/tmp/main.c")
	if cmd == "" {
		t.Error("expected non-empty build command for .c file")
	}
}

func TestFormatRunCommand(t *testing.T) {
	cmd := FormatRunCommand("/tmp/script.py")
	if cmd == "" {
		t.Error("expected non-empty run command for .py file")
	}
}

func TestFormatBuildCommand_NoBuild(t *testing.T) {
	cmd := FormatBuildCommand("/tmp/script.py")
	if cmd != "" {
		t.Errorf("expected empty build command for .py, got %s", cmd)
	}
}
