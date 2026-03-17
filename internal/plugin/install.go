package plugin

import (
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

const defaultRegistryURL = "https://raw.githubusercontent.com/numentech-co/plugin-registry/main/registry.json"

// RegistryEntry represents a plugin in the remote registry.
type RegistryEntry struct {
	Name        string   `json:"name"`
	Repo        string   `json:"repo"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	FileTypes   []string `json:"file_types,omitempty"`
}

// InstallPlugin clones a plugin repository into the plugins directory.
func InstallPlugin(pluginDir, repoURL string) error {
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return fmt.Errorf("creating plugin directory: %w", err)
	}

	// Derive plugin name from repo URL (last path segment without .git)
	name := repoNameFromURL(repoURL)
	if name == "" {
		return fmt.Errorf("cannot determine plugin name from URL: %s", repoURL)
	}

	destDir := filepath.Join(pluginDir, name)
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("plugin %q already installed at %s", name, destDir)
	}

	// Normalize repo URL: if it looks like github.com/user/repo, add https://
	cloneURL := repoURL
	if !strings.HasPrefix(cloneURL, "http://") && !strings.HasPrefix(cloneURL, "https://") && !strings.HasPrefix(cloneURL, "git@") {
		cloneURL = "https://" + cloneURL
	}

	cmd := exec.Command("git", "clone", "--depth", "1", cloneURL, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	// Verify plugin.json exists
	if _, err := os.Stat(filepath.Join(destDir, "plugin.json")); os.IsNotExist(err) {
		// Clean up invalid plugin
		_ = os.RemoveAll(destDir)
		return fmt.Errorf("cloned repository does not contain plugin.json")
	}

	fmt.Printf("Installed plugin %q to %s\n", name, destDir)
	return nil
}

// UpdatePlugin runs git pull in a plugin directory.
func UpdatePlugin(pluginDir, name string) error {
	dir := filepath.Join(pluginDir, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("plugin %q not found at %s", name, dir)
	}

	cmd := exec.Command("git", "-C", dir, "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	fmt.Printf("Updated plugin %q\n", name)
	return nil
}

// RemovePlugin deletes a plugin directory.
func RemovePlugin(pluginDir, name string) error {
	dir := filepath.Join(pluginDir, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("plugin %q not found at %s", name, dir)
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing plugin: %w", err)
	}

	fmt.Printf("Removed plugin %q\n", name)
	return nil
}

// ListPlugins lists all installed plugins.
func ListPlugins(pluginDir string) error {
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		fmt.Println("No plugins installed.")
		return nil
	}

	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return fmt.Errorf("reading plugin directory: %w", err)
	}

	found := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(pluginDir, entry.Name())
		manifest, err := LoadManifest(dir)
		if err != nil {
			fmt.Printf("  %s (invalid: %v)\n", entry.Name(), err)
			continue
		}
		fmt.Printf("  %s v%s - %s\n", manifest.Name, manifest.Version, manifest.Description)
		found = true
	}

	if !found {
		fmt.Println("No plugins installed.")
	}
	return nil
}

// SearchPlugins fetches the registry and searches for matching plugins.
func SearchPlugins(registryURL, keyword string) error {
	if registryURL == "" {
		registryURL = defaultRegistryURL
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(registryURL)
	if err != nil {
		return fmt.Errorf("fetching registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading registry response: %w", err)
	}

	var entries []RegistryEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return fmt.Errorf("parsing registry: %w", err)
	}

	keyword = strings.ToLower(keyword)
	found := false
	for _, e := range entries {
		if matchesKeyword(e, keyword) {
			fmt.Printf("  %s v%s - %s\n", e.Name, e.Version, e.Description)
			fmt.Printf("    Install: numentext --plugin-install %s\n", e.Repo)
			found = true
		}
	}

	if !found {
		fmt.Printf("No plugins matching %q found.\n", keyword)
	}
	return nil
}

func matchesKeyword(e RegistryEntry, keyword string) bool {
	if keyword == "" {
		return true
	}
	if strings.Contains(strings.ToLower(e.Name), keyword) {
		return true
	}
	if strings.Contains(strings.ToLower(e.Description), keyword) {
		return true
	}
	for _, ft := range e.FileTypes {
		if strings.Contains(strings.ToLower(ft), keyword) {
			return true
		}
	}
	return false
}

func repoNameFromURL(url string) string {
	// Handle git@github.com:user/repo.git and https://github.com/user/repo.git
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimSuffix(url, "/")

	// Split by / or :
	parts := strings.FieldsFunc(url, func(r rune) bool {
		return r == '/' || r == ':'
	})
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
