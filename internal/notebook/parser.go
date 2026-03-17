package notebook

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ipynbFile represents the top-level structure of a .ipynb file (Jupyter Notebook v4).
type ipynbFile struct {
	Cells    []ipynbCell    `json:"cells"`
	Metadata ipynbMetadata  `json:"metadata"`
	Nbformat int            `json:"nbformat"`
	NbfmtMin int            `json:"nbformat_minor"`
}

// ipynbMetadata holds notebook-level metadata.
type ipynbMetadata struct {
	KernelSpec  ipynbKernelSpec  `json:"kernelspec"`
	LangInfo    ipynbLangInfo    `json:"language_info"`
	Raw         json.RawMessage  `json:"-"` // preserves unknown fields
}

// ipynbKernelSpec describes the kernel.
type ipynbKernelSpec struct {
	DisplayName string `json:"display_name"`
	Language    string `json:"language"`
	Name        string `json:"name"`
}

// ipynbLangInfo describes the language.
type ipynbLangInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ipynbCell is a single notebook cell on disk.
type ipynbCell struct {
	CellType       string            `json:"cell_type"`
	Source         json.RawMessage   `json:"source"`
	Outputs        []ipynbOutput     `json:"outputs,omitempty"`
	ExecCount      *int              `json:"execution_count"`
	Metadata       json.RawMessage   `json:"metadata,omitempty"`
}

// ipynbOutput is one output entry for a code cell.
type ipynbOutput struct {
	OutputType string            `json:"output_type"`
	Name       string            `json:"name,omitempty"`       // for stream
	Text       json.RawMessage   `json:"text,omitempty"`       // string or []string
	Data       map[string]json.RawMessage `json:"data,omitempty"` // for display_data / execute_result
	ECount     *int              `json:"execution_count,omitempty"`
	Traceback  []string          `json:"traceback,omitempty"`
	EName      string            `json:"ename,omitempty"`
	EValue     string            `json:"evalue,omitempty"`
}

// Cell is the in-memory representation of a notebook cell.
type Cell struct {
	CellType  string   // "code", "markdown", "raw"
	Source    []string // source lines (without trailing newlines)
	Outputs  []Output // execution outputs
	ExecCount int      // execution counter (0 = not executed)
	Metadata  json.RawMessage // preserve original metadata

	// UI state (not serialized)
	Selected  bool
	EditMode  bool
	CursorRow int
	CursorCol int
}

// Output represents a single output block.
type Output struct {
	OutputType string   // "stream", "execute_result", "error", "display_data"
	Text       string   // for stream / execute_result
	ImageData  string   // base64 PNG for display_data
	ImagePath  string   // saved temp file path
	Traceback  []string // for error output
	StreamName string   // "stdout" or "stderr" for stream type
}

// Notebook is the parsed in-memory representation.
type Notebook struct {
	Cells    []*Cell
	Language string // kernel language (e.g. "python")
	KernelDisplay string
	KernelName    string
	Nbformat     int
	NbfmtMin     int
	Metadata     json.RawMessage // preserve full raw metadata for round-trip
}

// ParseNotebook parses .ipynb JSON data into a Notebook.
func ParseNotebook(data []byte) (*Notebook, error) {
	var raw ipynbFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid notebook JSON: %w", err)
	}
	if raw.Nbformat < 4 {
		// We only fully support v4, but try our best with older formats
		if raw.Nbformat == 0 && len(raw.Cells) == 0 {
			return nil, fmt.Errorf("not a valid Jupyter notebook (nbformat=%d)", raw.Nbformat)
		}
	}

	nb := &Notebook{
		Language:      raw.Metadata.KernelSpec.Language,
		KernelDisplay: raw.Metadata.KernelSpec.DisplayName,
		KernelName:    raw.Metadata.KernelSpec.Name,
		Nbformat:      raw.Nbformat,
		NbfmtMin:      raw.NbfmtMin,
	}

	// If language not in kernelspec, try language_info
	if nb.Language == "" {
		nb.Language = raw.Metadata.LangInfo.Name
	}

	// Preserve raw metadata for round-trip
	metaBytes, _ := json.Marshal(raw.Metadata)
	nb.Metadata = metaBytes
	// Re-parse the whole thing to keep unknown metadata fields
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawMap); err == nil {
		if m, ok := rawMap["metadata"]; ok {
			nb.Metadata = m
		}
	}

	for _, rc := range raw.Cells {
		cell := &Cell{
			CellType: rc.CellType,
			Metadata: rc.Metadata,
		}
		if rc.ExecCount != nil {
			cell.ExecCount = *rc.ExecCount
		}

		// Parse source: can be a string or an array of strings
		cell.Source = parseMultilineJSON(rc.Source)

		// Parse outputs
		for _, ro := range rc.Outputs {
			out := Output{
				OutputType: ro.OutputType,
				StreamName: ro.Name,
				Traceback:  ro.Traceback,
			}
			switch ro.OutputType {
			case "stream":
				out.Text = parseTextJSON(ro.Text)
			case "execute_result", "display_data":
				if textRaw, ok := ro.Data["text/plain"]; ok {
					out.Text = parseTextJSON(textRaw)
				}
				if imgRaw, ok := ro.Data["image/png"]; ok {
					var imgStr string
					if err := json.Unmarshal(imgRaw, &imgStr); err == nil {
						out.ImageData = imgStr
					}
				}
			case "error":
				out.Text = ro.EName + ": " + ro.EValue
			}
			cell.Outputs = append(cell.Outputs, out)
		}

		nb.Cells = append(nb.Cells, cell)
	}

	return nb, nil
}

// SerializeNotebook converts a Notebook back to .ipynb JSON.
func SerializeNotebook(nb *Notebook) ([]byte, error) {
	raw := ipynbFile{
		Nbformat: nb.Nbformat,
		NbfmtMin: nb.NbfmtMin,
	}
	if raw.Nbformat == 0 {
		raw.Nbformat = 4
		raw.NbfmtMin = 5
	}

	// Rebuild metadata from preserved raw if possible
	// We build a map and inject kernelspec/language_info
	rawMeta := make(map[string]json.RawMessage)
	if len(nb.Metadata) > 0 {
		_ = json.Unmarshal(nb.Metadata, &rawMeta)
	}
	// Ensure kernelspec is present
	ks := ipynbKernelSpec{
		DisplayName: nb.KernelDisplay,
		Language:    nb.Language,
		Name:        nb.KernelName,
	}
	ksBytes, _ := json.Marshal(ks)
	rawMeta["kernelspec"] = ksBytes
	metaBytes, _ := json.Marshal(rawMeta)
	if err := json.Unmarshal(metaBytes, &raw.Metadata); err != nil {
		// Fallback
		raw.Metadata.KernelSpec = ks
	}

	for _, cell := range nb.Cells {
		rc := ipynbCell{
			CellType: cell.CellType,
			Metadata: cell.Metadata,
		}
		if rc.Metadata == nil {
			rc.Metadata = json.RawMessage("{}")
		}
		if cell.ExecCount > 0 {
			ec := cell.ExecCount
			rc.ExecCount = &ec
		}

		// Serialize source as array of strings (Jupyter convention)
		rc.Source = sourceToJSON(cell.Source)

		// Serialize outputs
		if cell.CellType == "code" {
			rc.Outputs = make([]ipynbOutput, 0, len(cell.Outputs))
			for _, out := range cell.Outputs {
				ro := ipynbOutput{
					OutputType: out.OutputType,
				}
				switch out.OutputType {
				case "stream":
					ro.Name = out.StreamName
					if ro.Name == "" {
						ro.Name = "stdout"
					}
					ro.Text = textToJSON(out.Text)
				case "execute_result", "display_data":
					ro.Data = make(map[string]json.RawMessage)
					if out.Text != "" {
						ro.Data["text/plain"] = textToJSON(out.Text)
					}
					if out.ImageData != "" {
						imgBytes, _ := json.Marshal(out.ImageData)
						ro.Data["image/png"] = imgBytes
					}
					if out.OutputType == "execute_result" {
						ec := cell.ExecCount
						ro.ECount = &ec
					}
				case "error":
					ro.Traceback = out.Traceback
					parts := strings.SplitN(out.Text, ": ", 2)
					if len(parts) == 2 {
						ro.EName = parts[0]
						ro.EValue = parts[1]
					} else {
						ro.EName = "Error"
						ro.EValue = out.Text
					}
				}
				rc.Outputs = append(rc.Outputs, ro)
			}
		}

		raw.Cells = append(raw.Cells, rc)
	}

	return json.MarshalIndent(raw, "", " ")
}

// parseMultilineJSON parses a JSON value that can be either a string or []string.
// Returns lines without trailing newlines.
func parseMultilineJSON(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}

	// Try as string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		lines := strings.Split(s, "\n")
		// Remove trailing empty line from final newline
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		return lines
	}

	// Try as array of strings
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		var result []string
		for _, line := range arr {
			// Lines in the array typically end with \n
			line = strings.TrimRight(line, "\n")
			result = append(result, line)
		}
		return result
	}

	return nil
}

// parseTextJSON parses text from a JSON value (string or []string) into a single string.
func parseTextJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, "")
	}
	return ""
}

// sourceToJSON converts source lines to JSON array format (Jupyter convention).
func sourceToJSON(lines []string) json.RawMessage {
	if len(lines) == 0 {
		arr := make([]string, 0)
		b, _ := json.Marshal(arr)
		return b
	}
	// Each line except the last gets a trailing \n
	arr := make([]string, len(lines))
	for i, line := range lines {
		if i < len(lines)-1 {
			arr[i] = line + "\n"
		} else {
			arr[i] = line
		}
	}
	b, _ := json.Marshal(arr)
	return b
}

// textToJSON converts a text string into a JSON array of lines (Jupyter convention).
func textToJSON(text string) json.RawMessage {
	lines := strings.Split(text, "\n")
	arr := make([]string, len(lines))
	for i, line := range lines {
		if i < len(lines)-1 {
			arr[i] = line + "\n"
		} else {
			arr[i] = line
		}
	}
	b, _ := json.Marshal(arr)
	return b
}

// IsNotebookFile returns true if the file path has a .ipynb extension.
func IsNotebookFile(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".ipynb")
}
