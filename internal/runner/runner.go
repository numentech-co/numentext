package runner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Result holds the outcome of a run/build
type Result struct {
	Command  string
	Output   string
	Error    string
	ExitCode int
	Duration time.Duration
}

// Runner orchestrates compile and run operations
type Runner struct {
	cancelFunc context.CancelFunc
	running    bool
}

func New() *Runner {
	return &Runner{}
}

func (r *Runner) IsRunning() bool {
	return r.running
}

// Build compiles the file
func (r *Runner) Build(filePath string) *Result {
	lang := DetectLanguage(filePath)
	if lang == nil {
		return &Result{Error: "unsupported language"}
	}
	if lang.BuildCmd == nil {
		return &Result{Output: fmt.Sprintf("%s does not require compilation", lang.Name)}
	}

	cmdName, args := lang.BuildCmd(filePath)
	return r.execute(cmdName, args)
}

// Run executes the file (building first if needed)
func (r *Runner) Run(filePath string) *Result {
	lang := DetectLanguage(filePath)
	if lang == nil {
		return &Result{Error: "unsupported language"}
	}

	// Build first if needed
	if lang.BuildCmd != nil {
		buildResult := r.Build(filePath)
		if buildResult.Error != "" || buildResult.ExitCode != 0 {
			return buildResult
		}
	}

	if lang.RunCmd == nil {
		return &Result{Error: "no run command configured"}
	}

	cmdName, args := lang.RunCmd(filePath)
	return r.execute(cmdName, args)
}

// RunOnly runs without building
func (r *Runner) RunOnly(filePath string) *Result {
	lang := DetectLanguage(filePath)
	if lang == nil {
		return &Result{Error: "unsupported language"}
	}
	if lang.RunCmd == nil {
		return &Result{Error: "no run command configured"}
	}

	cmdName, args := lang.RunCmd(filePath)
	return r.execute(cmdName, args)
}

// Stop stops the currently running process
func (r *Runner) Stop() {
	if r.cancelFunc != nil {
		r.cancelFunc()
	}
}

func (r *Runner) execute(cmdName string, args []string) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	r.cancelFunc = cancel
	r.running = true
	defer func() {
		r.running = false
		cancel()
	}()

	fullCmd := cmdName
	if len(args) > 0 {
		fullCmd += " " + strings.Join(args, " ")
	}

	start := time.Now()
	cmd := exec.CommandContext(ctx, cmdName, args...)

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := &Result{
		Command:  fullCmd,
		Output:   string(output),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			result.Error = string(output)
		} else {
			result.Error = err.Error()
			result.ExitCode = -1
		}
	}

	return result
}

// FormatCommand returns a display string for the build/run command
func FormatBuildCommand(filePath string) string {
	lang := DetectLanguage(filePath)
	if lang == nil || lang.BuildCmd == nil {
		return ""
	}
	cmd, args := lang.BuildCmd(filePath)
	if len(args) > 0 {
		return cmd + " " + strings.Join(args, " ")
	}
	return cmd
}

func FormatRunCommand(filePath string) string {
	lang := DetectLanguage(filePath)
	if lang == nil || lang.RunCmd == nil {
		return ""
	}
	cmd, args := lang.RunCmd(filePath)
	if len(args) > 0 {
		return cmd + " " + strings.Join(args, " ")
	}
	return cmd
}
