package plugin

import (
	"testing"
)

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://github.com/user/my-plugin.git", "my-plugin"},
		{"https://github.com/user/my-plugin", "my-plugin"},
		{"github.com/user/my-plugin", "my-plugin"},
		{"git@github.com:user/my-plugin.git", "my-plugin"},
		{"https://github.com/user/repo/", "repo"},
		{"", ""},
	}

	for _, tt := range tests {
		got := repoNameFromURL(tt.url)
		if got != tt.want {
			t.Errorf("repoNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestMatchesKeyword(t *testing.T) {
	entry := RegistryEntry{
		Name:        "notebook-viewer",
		Description: "View Jupyter notebooks in NumenText",
		FileTypes:   []string{".ipynb"},
	}

	if !matchesKeyword(entry, "notebook") {
		t.Error("expected match on name")
	}
	if !matchesKeyword(entry, "jupyter") {
		t.Error("expected match on description")
	}
	if !matchesKeyword(entry, "ipynb") {
		t.Error("expected match on file type")
	}
	if matchesKeyword(entry, "debugger") {
		t.Error("expected no match")
	}
	if !matchesKeyword(entry, "") {
		t.Error("empty keyword should match everything")
	}
}
