package notebook

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Executor handles cell execution, either via Jupyter REST API or subprocess fallback.
type Executor struct {
	workDir    string
	language   string
	jupyterURL string // e.g. "http://localhost:8888"
	token      string // Jupyter auth token
	kernelID   string // active kernel ID for REST API
}

// NewExecutor creates a new cell executor.
func NewExecutor(workDir, language string) *Executor {
	return &Executor{
		workDir:  workDir,
		language: language,
	}
}

// DetectJupyter tries to find a running Jupyter server on common ports.
func (e *Executor) DetectJupyter() bool {
	ports := []string{"8888", "8889", "8890"}
	for _, port := range ports {
		url := "http://localhost:" + port
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(url + "/api")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				e.jupyterURL = url
				return true
			}
		}
	}
	return false
}

// SetJupyterURL sets the Jupyter server URL and optional token.
func (e *Executor) SetJupyterURL(url, token string) {
	e.jupyterURL = url
	e.token = token
}

// ExecuteCell executes a single code cell and returns the output.
// Uses Jupyter REST API if available, otherwise falls back to subprocess.
func (e *Executor) ExecuteCell(code string) (*Output, error) {
	if e.jupyterURL != "" {
		return e.executeViaJupyter(code)
	}
	return e.executeViaSubprocess(code)
}

// executeViaSubprocess runs code through a language interpreter.
func (e *Executor) executeViaSubprocess(code string) (*Output, error) {
	var cmd *exec.Cmd

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch strings.ToLower(e.language) {
	case "python", "python3", "":
		cmd = exec.CommandContext(ctx, "python3", "-c", code)
	case "r":
		cmd = exec.CommandContext(ctx, "Rscript", "-e", code)
	case "julia":
		cmd = exec.CommandContext(ctx, "julia", "-e", code)
	default:
		// Try python3 as default
		cmd = exec.CommandContext(ctx, "python3", "-c", code)
	}

	cmd.Dir = e.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	out := &Output{
		OutputType: "stream",
		StreamName: "stdout",
	}

	if stdout.Len() > 0 {
		out.Text = stdout.String()
	}

	if stderr.Len() > 0 {
		if err != nil {
			// Execution error
			out.OutputType = "error"
			out.Text = "Error: " + stderr.String()
			out.Traceback = strings.Split(stderr.String(), "\n")
		} else {
			// Stderr output but no error (warnings etc)
			if out.Text != "" {
				out.Text += "\n"
			}
			out.Text += stderr.String()
		}
	} else if err != nil {
		out.OutputType = "error"
		out.Text = "Error: " + err.Error()
		out.Traceback = []string{err.Error()}
	}

	// Check for matplotlib-style image output
	// If the code produces a PNG file in a known temp location, detect it
	out.ImagePath = e.detectImageOutput(code)

	return out, nil
}

// detectImageOutput checks if execution produced an image file.
func (e *Executor) detectImageOutput(code string) string {
	// Simple heuristic: if code mentions plt.show() or plt.savefig(),
	// check for saved images. For now we do not auto-detect.
	_ = code
	return ""
}

// executeViaJupyter executes code via the Jupyter REST API.
func (e *Executor) executeViaJupyter(code string) (*Output, error) {
	if e.kernelID == "" {
		id, err := e.startKernel()
		if err != nil {
			// Fall back to subprocess
			return e.executeViaSubprocess(code)
		}
		e.kernelID = id
	}

	// Use the /api/kernels/{id}/execute endpoint (available in newer Jupyter)
	// or fall back to nbconvert approach
	return e.executeViaKernelAPI(code)
}

// startKernel starts a new kernel via Jupyter REST API.
func (e *Executor) startKernel() (string, error) {
	body := fmt.Sprintf(`{"name": "%s"}`, e.kernelName())
	req, err := http.NewRequest("POST", e.jupyterURL+"/api/kernels", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.token != "" {
		req.Header.Set("Authorization", "token "+e.token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to start kernel: HTTP %d", resp.StatusCode)
	}

	var result struct {
		ID string `json:"id"`
	}
	respBody, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	return result.ID, nil
}

func (e *Executor) kernelName() string {
	switch strings.ToLower(e.language) {
	case "python", "python3", "":
		return "python3"
	case "r":
		return "ir"
	case "julia":
		return "julia-1.9"
	default:
		return "python3"
	}
}

// executeViaKernelAPI uses nbconvert --execute as a reliable execution method.
func (e *Executor) executeViaKernelAPI(code string) (*Output, error) {
	// Create a minimal notebook with just this cell
	tmpNb := map[string]interface{}{
		"nbformat":       4,
		"nbformat_minor": 5,
		"metadata": map[string]interface{}{
			"kernelspec": map[string]string{
				"name":     e.kernelName(),
				"language": e.language,
			},
		},
		"cells": []map[string]interface{}{
			{
				"cell_type": "code",
				"source":    code,
				"metadata":  map[string]interface{}{},
				"outputs":   []interface{}{},
			},
		},
	}

	tmpData, err := json.Marshal(tmpNb)
	if err != nil {
		return nil, err
	}

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("numentext-exec-%d.ipynb", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, tmpData, 0644); err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile)

	outFile := tmpFile + ".out.ipynb"
	defer os.Remove(outFile)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "jupyter", "nbconvert",
		"--to", "notebook",
		"--execute",
		"--output", outFile,
		tmpFile)
	cmd.Dir = e.workDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &Output{
			OutputType: "error",
			Text:       "Error: " + stderr.String(),
			Traceback:  strings.Split(stderr.String(), "\n"),
		}, nil
	}

	// Parse the executed notebook to extract outputs
	outData, err := os.ReadFile(outFile)
	if err != nil {
		return nil, err
	}

	nb, err := ParseNotebook(outData)
	if err != nil {
		return nil, err
	}

	if len(nb.Cells) > 0 && len(nb.Cells[0].Outputs) > 0 {
		return &nb.Cells[0].Outputs[0], nil
	}

	return &Output{
		OutputType: "stream",
		StreamName: "stdout",
		Text:       "",
	}, nil
}

// ShutdownKernel shuts down the active kernel if using Jupyter API.
func (e *Executor) ShutdownKernel() {
	if e.jupyterURL == "" || e.kernelID == "" {
		return
	}

	req, err := http.NewRequest("DELETE", e.jupyterURL+"/api/kernels/"+e.kernelID, nil)
	if err != nil {
		return
	}
	if e.token != "" {
		req.Header.Set("Authorization", "token "+e.token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	e.kernelID = ""
}

// SaveImageOutput decodes base64 image data and saves it to a temp file.
// Returns the file path.
func SaveImageOutput(imageData string, index int) (string, error) {
	data, err := base64.StdEncoding.DecodeString(imageData)
	if err != nil {
		return "", fmt.Errorf("invalid base64 image data: %w", err)
	}

	path := filepath.Join(os.TempDir(), fmt.Sprintf("numentext-plot-%03d.png", index))
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}

	return path, nil
}
