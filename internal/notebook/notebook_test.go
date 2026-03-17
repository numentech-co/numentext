package notebook

import (
	"encoding/json"
	"strings"
	"testing"
)

// Minimal valid Jupyter notebook v4 JSON for testing.
const testNotebookJSON = `{
 "nbformat": 4,
 "nbformat_minor": 5,
 "metadata": {
  "kernelspec": {
   "display_name": "Python 3",
   "language": "python",
   "name": "python3"
  },
  "language_info": {
   "name": "python",
   "version": "3.10.0"
  }
 },
 "cells": [
  {
   "cell_type": "markdown",
   "source": ["# Hello\n", "This is markdown."],
   "metadata": {}
  },
  {
   "cell_type": "code",
   "source": ["import os\n", "print(os.getcwd())"],
   "metadata": {},
   "execution_count": 1,
   "outputs": [
    {
     "output_type": "stream",
     "name": "stdout",
     "text": ["/home/user\n"]
    }
   ]
  },
  {
   "cell_type": "code",
   "source": "x = 42",
   "metadata": {},
   "execution_count": 2,
   "outputs": [
    {
     "output_type": "execute_result",
     "execution_count": 2,
     "data": {
      "text/plain": "42"
     },
     "metadata": {}
    }
   ]
  },
  {
   "cell_type": "code",
   "source": ["raise ValueError('oops')"],
   "metadata": {},
   "execution_count": 3,
   "outputs": [
    {
     "output_type": "error",
     "ename": "ValueError",
     "evalue": "oops",
     "traceback": ["Traceback:", "ValueError: oops"]
    }
   ]
  },
  {
   "cell_type": "raw",
   "source": ["raw text"],
   "metadata": {}
  }
 ]
}`

func TestParseNotebook(t *testing.T) {
	nb, err := ParseNotebook([]byte(testNotebookJSON))
	if err != nil {
		t.Fatalf("ParseNotebook: %v", err)
	}

	if nb.Nbformat != 4 {
		t.Errorf("nbformat = %d, want 4", nb.Nbformat)
	}
	if nb.Language != "python" {
		t.Errorf("language = %q, want %q", nb.Language, "python")
	}
	if nb.KernelDisplay != "Python 3" {
		t.Errorf("kernel display = %q, want %q", nb.KernelDisplay, "Python 3")
	}

	if len(nb.Cells) != 5 {
		t.Fatalf("cell count = %d, want 5", len(nb.Cells))
	}

	// Cell 0: markdown
	c0 := nb.Cells[0]
	if c0.CellType != "markdown" {
		t.Errorf("cell[0] type = %q, want markdown", c0.CellType)
	}
	if len(c0.Source) != 2 {
		t.Errorf("cell[0] source lines = %d, want 2", len(c0.Source))
	}
	if c0.Source[0] != "# Hello" {
		t.Errorf("cell[0] source[0] = %q, want %q", c0.Source[0], "# Hello")
	}

	// Cell 1: code with stream output
	c1 := nb.Cells[1]
	if c1.CellType != "code" {
		t.Errorf("cell[1] type = %q, want code", c1.CellType)
	}
	if c1.ExecCount != 1 {
		t.Errorf("cell[1] exec_count = %d, want 1", c1.ExecCount)
	}
	if len(c1.Outputs) != 1 {
		t.Fatalf("cell[1] outputs = %d, want 1", len(c1.Outputs))
	}
	if c1.Outputs[0].OutputType != "stream" {
		t.Errorf("cell[1] output type = %q, want stream", c1.Outputs[0].OutputType)
	}
	if !strings.Contains(c1.Outputs[0].Text, "/home/user") {
		t.Errorf("cell[1] output text = %q, want to contain /home/user", c1.Outputs[0].Text)
	}

	// Cell 2: code with execute_result
	c2 := nb.Cells[2]
	if c2.ExecCount != 2 {
		t.Errorf("cell[2] exec_count = %d, want 2", c2.ExecCount)
	}
	if len(c2.Outputs) != 1 || c2.Outputs[0].Text != "42" {
		t.Errorf("cell[2] output text = %q, want %q", c2.Outputs[0].Text, "42")
	}

	// Cell 3: code with error
	c3 := nb.Cells[3]
	if len(c3.Outputs) != 1 || c3.Outputs[0].OutputType != "error" {
		t.Fatalf("cell[3] output not error")
	}
	if !strings.Contains(c3.Outputs[0].Text, "ValueError") {
		t.Errorf("cell[3] error text = %q, want ValueError", c3.Outputs[0].Text)
	}
	if len(c3.Outputs[0].Traceback) != 2 {
		t.Errorf("cell[3] traceback len = %d, want 2", len(c3.Outputs[0].Traceback))
	}

	// Cell 4: raw
	c4 := nb.Cells[4]
	if c4.CellType != "raw" {
		t.Errorf("cell[4] type = %q, want raw", c4.CellType)
	}
}

func TestParseNotebookSourceAsString(t *testing.T) {
	// Test that source can be a plain string (not array)
	nb, err := ParseNotebook([]byte(testNotebookJSON))
	if err != nil {
		t.Fatalf("ParseNotebook: %v", err)
	}
	// Cell 2 has source as a plain string
	c2 := nb.Cells[2]
	if len(c2.Source) != 1 || c2.Source[0] != "x = 42" {
		t.Errorf("cell[2] source = %v, want [\"x = 42\"]", c2.Source)
	}
}

func TestParseNotebookInvalid(t *testing.T) {
	_, err := ParseNotebook([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseNotebookEmpty(t *testing.T) {
	data := `{"nbformat": 4, "nbformat_minor": 5, "metadata": {}, "cells": []}`
	nb, err := ParseNotebook([]byte(data))
	if err != nil {
		t.Fatalf("ParseNotebook: %v", err)
	}
	if len(nb.Cells) != 0 {
		t.Errorf("cell count = %d, want 0", len(nb.Cells))
	}
}

func TestSerializeNotebook(t *testing.T) {
	nb, err := ParseNotebook([]byte(testNotebookJSON))
	if err != nil {
		t.Fatalf("ParseNotebook: %v", err)
	}

	data, err := SerializeNotebook(nb)
	if err != nil {
		t.Fatalf("SerializeNotebook: %v", err)
	}

	// Parse it back and verify
	nb2, err := ParseNotebook(data)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	if len(nb2.Cells) != len(nb.Cells) {
		t.Fatalf("cell count mismatch: %d vs %d", len(nb2.Cells), len(nb.Cells))
	}

	for i, c := range nb2.Cells {
		orig := nb.Cells[i]
		if c.CellType != orig.CellType {
			t.Errorf("cell[%d] type = %q, want %q", i, c.CellType, orig.CellType)
		}
		if len(c.Source) != len(orig.Source) {
			t.Errorf("cell[%d] source lines = %d, want %d", i, len(c.Source), len(orig.Source))
		}
		for j := range c.Source {
			if c.Source[j] != orig.Source[j] {
				t.Errorf("cell[%d] source[%d] = %q, want %q", i, j, c.Source[j], orig.Source[j])
			}
		}
	}
}

func TestSerializePreservesMetadata(t *testing.T) {
	nb, err := ParseNotebook([]byte(testNotebookJSON))
	if err != nil {
		t.Fatalf("ParseNotebook: %v", err)
	}

	data, err := SerializeNotebook(nb)
	if err != nil {
		t.Fatalf("SerializeNotebook: %v", err)
	}

	// Check that kernelspec is preserved
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	var meta map[string]json.RawMessage
	if err := json.Unmarshal(raw["metadata"], &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}

	if _, ok := meta["kernelspec"]; !ok {
		t.Error("missing kernelspec in metadata")
	}
}

func TestSerializeOutputs(t *testing.T) {
	nb, err := ParseNotebook([]byte(testNotebookJSON))
	if err != nil {
		t.Fatalf("ParseNotebook: %v", err)
	}

	data, err := SerializeNotebook(nb)
	if err != nil {
		t.Fatalf("SerializeNotebook: %v", err)
	}

	// Re-parse and check outputs are preserved
	nb2, err := ParseNotebook(data)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}

	// Cell 1 should have stream output
	if len(nb2.Cells[1].Outputs) != 1 {
		t.Fatalf("cell[1] outputs = %d, want 1", len(nb2.Cells[1].Outputs))
	}
	if nb2.Cells[1].Outputs[0].OutputType != "stream" {
		t.Errorf("cell[1] output type = %q, want stream", nb2.Cells[1].Outputs[0].OutputType)
	}

	// Cell 3 should have error output
	if len(nb2.Cells[3].Outputs) != 1 {
		t.Fatalf("cell[3] outputs = %d, want 1", len(nb2.Cells[3].Outputs))
	}
	if nb2.Cells[3].Outputs[0].OutputType != "error" {
		t.Errorf("cell[3] output type = %q, want error", nb2.Cells[3].Outputs[0].OutputType)
	}
}

func TestCellManipulation(t *testing.T) {
	nb, err := ParseNotebook([]byte(testNotebookJSON))
	if err != nil {
		t.Fatalf("ParseNotebook: %v", err)
	}

	origCount := len(nb.Cells)

	// Insert a cell
	newCell := &Cell{
		CellType: "code",
		Source:   []string{"print('new')"},
	}
	nb.Cells = append(nb.Cells[:2], append([]*Cell{newCell}, nb.Cells[2:]...)...)
	if len(nb.Cells) != origCount+1 {
		t.Errorf("after insert: count = %d, want %d", len(nb.Cells), origCount+1)
	}

	// Delete a cell
	nb.Cells = append(nb.Cells[:2], nb.Cells[3:]...)
	if len(nb.Cells) != origCount {
		t.Errorf("after delete: count = %d, want %d", len(nb.Cells), origCount)
	}

	// Toggle type
	nb.Cells[0].CellType = "code"
	if nb.Cells[0].CellType != "code" {
		t.Errorf("after toggle: type = %q, want code", nb.Cells[0].CellType)
	}
}

func TestIsNotebookFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"foo.ipynb", true},
		{"dir/bar.IPYNB", true},
		{"foo.py", false},
		{"notebook.ipynb.bak", false},
		{"test.json", false},
		{"", false},
	}

	for _, tt := range tests {
		got := IsNotebookFile(tt.path)
		if got != tt.want {
			t.Errorf("IsNotebookFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestParseMultilineJSON(t *testing.T) {
	// String form
	raw := json.RawMessage(`"line1\nline2\n"`)
	lines := parseMultilineJSON(raw)
	if len(lines) != 2 {
		t.Fatalf("string form: lines = %d, want 2", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" {
		t.Errorf("string form: lines = %v", lines)
	}

	// Array form
	raw = json.RawMessage(`["line1\n", "line2"]`)
	lines = parseMultilineJSON(raw)
	if len(lines) != 2 {
		t.Fatalf("array form: lines = %d, want 2", len(lines))
	}
	if lines[0] != "line1" || lines[1] != "line2" {
		t.Errorf("array form: lines = %v", lines)
	}
}

func TestSourceToJSON(t *testing.T) {
	lines := []string{"import os", "print(os.getcwd())"}
	raw := sourceToJSON(lines)

	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(arr) != 2 {
		t.Fatalf("arr len = %d, want 2", len(arr))
	}
	// First line should have trailing \n, last should not
	if arr[0] != "import os\n" {
		t.Errorf("arr[0] = %q, want %q", arr[0], "import os\n")
	}
	if arr[1] != "print(os.getcwd())" {
		t.Errorf("arr[1] = %q, want %q", arr[1], "print(os.getcwd())")
	}
}

func TestSourceToJSONEmpty(t *testing.T) {
	raw := sourceToJSON(nil)
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(arr) != 0 {
		t.Errorf("arr len = %d, want 0", len(arr))
	}
}

func TestImageOutput(t *testing.T) {
	// Notebook with a display_data image output
	nbJSON := `{
 "nbformat": 4,
 "nbformat_minor": 5,
 "metadata": {"kernelspec": {"language": "python"}},
 "cells": [
  {
   "cell_type": "code",
   "source": ["import matplotlib"],
   "metadata": {},
   "execution_count": 1,
   "outputs": [
    {
     "output_type": "display_data",
     "data": {
      "text/plain": "<Figure>",
      "image/png": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
     },
     "metadata": {}
    }
   ]
  }
 ]
}`

	nb, err := ParseNotebook([]byte(nbJSON))
	if err != nil {
		t.Fatalf("ParseNotebook: %v", err)
	}

	if len(nb.Cells) != 1 {
		t.Fatalf("cell count = %d, want 1", len(nb.Cells))
	}

	cell := nb.Cells[0]
	if len(cell.Outputs) != 1 {
		t.Fatalf("output count = %d, want 1", len(cell.Outputs))
	}

	out := cell.Outputs[0]
	if out.OutputType != "display_data" {
		t.Errorf("output type = %q, want display_data", out.OutputType)
	}
	if out.ImageData == "" {
		t.Error("expected image data to be populated")
	}
	if out.Text != "<Figure>" {
		t.Errorf("text = %q, want %q", out.Text, "<Figure>")
	}
}

func TestRoundTrip(t *testing.T) {
	// Parse, serialize, parse again, and verify equivalence
	nb1, err := ParseNotebook([]byte(testNotebookJSON))
	if err != nil {
		t.Fatalf("parse 1: %v", err)
	}

	data, err := SerializeNotebook(nb1)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	nb2, err := ParseNotebook(data)
	if err != nil {
		t.Fatalf("parse 2: %v", err)
	}

	if nb2.Nbformat != nb1.Nbformat {
		t.Errorf("nbformat mismatch: %d vs %d", nb2.Nbformat, nb1.Nbformat)
	}
	if nb2.Language != nb1.Language {
		t.Errorf("language mismatch: %q vs %q", nb2.Language, nb1.Language)
	}
	if len(nb2.Cells) != len(nb1.Cells) {
		t.Fatalf("cell count mismatch: %d vs %d", len(nb2.Cells), len(nb1.Cells))
	}

	for i := range nb1.Cells {
		if nb2.Cells[i].CellType != nb1.Cells[i].CellType {
			t.Errorf("cell[%d] type mismatch: %q vs %q", i, nb2.Cells[i].CellType, nb1.Cells[i].CellType)
		}
		if nb2.Cells[i].ExecCount != nb1.Cells[i].ExecCount {
			t.Errorf("cell[%d] exec count mismatch: %d vs %d", i, nb2.Cells[i].ExecCount, nb1.Cells[i].ExecCount)
		}
	}
}
