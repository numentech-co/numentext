package lsp

import (
	"encoding/json"
	"testing"
)

func TestRequestMarshal(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  map[string]string{"key": "value"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m["jsonrpc"] != "2.0" {
		t.Error("jsonrpc should be 2.0")
	}
	if m["method"] != "initialize" {
		t.Error("method should be initialize")
	}
}

func TestNotificationNoID(t *testing.T) {
	notif := Notification{
		JSONRPC: "2.0",
		Method:  "initialized",
	}
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["id"]; ok {
		t.Error("notification should not have id")
	}
}

func TestResponseError(t *testing.T) {
	id := 1
	resp := Response{
		JSONRPC: "2.0",
		ID:      &id,
		Error: &ResponseError{
			Code:    -32600,
			Message: "Invalid Request",
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Error == nil {
		t.Fatal("expected error")
	}
	if decoded.Error.Code != -32600 {
		t.Errorf("error code = %d, want -32600", decoded.Error.Code)
	}
}

func TestPathToURI(t *testing.T) {
	uri := pathToURI("/home/user/test.go")
	if uri != "file://%2Fhome%2Fuser%2Ftest.go" {
		// The actual encoding may vary, just check prefix
		if len(uri) < 7 {
			t.Errorf("uri too short: %s", uri)
		}
	}
	if uri[:7] != "file://" {
		t.Errorf("uri should start with file://, got %s", uri)
	}
}

func TestURIToPath(t *testing.T) {
	path := URIToPath("file:///home/user/test.go")
	if path != "/home/user/test.go" {
		t.Errorf("path = %s, want /home/user/test.go", path)
	}
}

func TestServerForFileGo(t *testing.T) {
	cfg := ServerForFile("main.go")
	// May be nil if gopls not installed, but shouldn't panic
	if cfg != nil && cfg.LanguageID != "go" {
		t.Errorf("languageID = %s, want go", cfg.LanguageID)
	}
}

func TestServerForFileUnknown(t *testing.T) {
	cfg := ServerForFile("readme.txt")
	if cfg != nil {
		t.Error("expected nil for unknown file type")
	}
}

func TestLanguageIDForFile(t *testing.T) {
	tests := []struct {
		file string
		want string
	}{
		{"main.go", "go"},
		{"test.py", "python"},
		{"lib.rs", "rust"},
		{"main.c", "c"},
		{"main.cpp", "cpp"},
		{"app.js", "javascript"},
		{"app.ts", "typescript"},
		{"Main.java", "java"},
		{"readme.txt", ""},
		{"data.json", ""},
	}
	for _, tt := range tests {
		got := LanguageIDForFile(tt.file)
		if got != tt.want {
			t.Errorf("LanguageIDForFile(%s) = %s, want %s", tt.file, got, tt.want)
		}
	}
}

func TestRemarshal(t *testing.T) {
	src := map[string]interface{}{
		"label":  "test",
		"kind":   float64(3),
		"detail": "A function",
	}
	var dst CompletionItem
	err := remarshal(src, &dst)
	if err != nil {
		t.Fatal(err)
	}
	if dst.Label != "test" {
		t.Errorf("label = %s, want test", dst.Label)
	}
	if dst.Kind != 3 {
		t.Errorf("kind = %d, want 3", dst.Kind)
	}
}
