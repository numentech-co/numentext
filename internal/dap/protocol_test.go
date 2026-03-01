package dap

import (
	"encoding/json"
	"testing"
)

func TestRequestMarshal(t *testing.T) {
	req := Request{
		Seq:     1,
		Type:    "request",
		Command: "initialize",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	if m["type"] != "request" {
		t.Error("type should be request")
	}
	if m["command"] != "initialize" {
		t.Error("command should be initialize")
	}
}

func TestEventMarshal(t *testing.T) {
	ev := Event{
		Seq:   1,
		Type:  "event",
		Event: "stopped",
		Body:  StoppedEventBody{Reason: "breakpoint", ThreadID: 1},
	}
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	_ = json.Unmarshal(data, &m)
	if m["event"] != "stopped" {
		t.Error("event should be stopped")
	}
}

func TestAdapterForFileGo(t *testing.T) {
	cfg := AdapterForFile("main.go")
	// May be nil if dlv not installed
	if cfg != nil && cfg.LanguageID != "go" {
		t.Errorf("languageID = %s, want go", cfg.LanguageID)
	}
}

func TestAdapterForFileUnknown(t *testing.T) {
	cfg := AdapterForFile("readme.txt")
	if cfg != nil {
		t.Error("expected nil for unknown file type")
	}
}

func TestManagerBreakpoints(t *testing.T) {
	m := NewManager()

	// Toggle on
	m.ToggleBreakpoint("test.go", 10)
	if !m.HasBreakpoint("test.go", 10) {
		t.Error("should have breakpoint at line 10")
	}

	// Toggle off
	m.ToggleBreakpoint("test.go", 10)
	if m.HasBreakpoint("test.go", 10) {
		t.Error("breakpoint at line 10 should be removed")
	}

	// Multiple breakpoints
	m.ToggleBreakpoint("test.go", 5)
	m.ToggleBreakpoint("test.go", 15)
	bps := m.Breakpoints("test.go")
	if len(bps) != 2 {
		t.Errorf("expected 2 breakpoints, got %d", len(bps))
	}
}

func TestManagerState(t *testing.T) {
	m := NewManager()
	if m.State() != SessionInactive {
		t.Error("initial state should be inactive")
	}
}

func TestRemarshal(t *testing.T) {
	src := map[string]interface{}{
		"verified": true,
		"line":     float64(10),
	}
	var dst Breakpoint
	err := remarshal(src, &dst)
	if err != nil {
		t.Fatal(err)
	}
	if !dst.Verified {
		t.Error("expected verified=true")
	}
	if dst.Line != 10 {
		t.Errorf("line = %d, want 10", dst.Line)
	}
}
