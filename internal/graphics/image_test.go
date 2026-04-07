package graphics

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// createTestPNG writes a small solid-color PNG to the given path.
func createTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePath_Absolute(t *testing.T) {
	got := ResolvePath("/foo/bar/img.png", "/base")
	if got != "/foo/bar/img.png" {
		t.Errorf("expected /foo/bar/img.png, got %s", got)
	}
}

func TestResolvePath_Relative(t *testing.T) {
	got := ResolvePath("images/photo.png", "/docs/project")
	want := filepath.Clean("/docs/project/images/photo.png")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestResolvePath_DotDot(t *testing.T) {
	got := ResolvePath("../assets/logo.png", "/docs/project")
	want := filepath.Clean("/docs/assets/logo.png")
	if got != want {
		t.Errorf("expected %s, got %s", want, got)
	}
}

func TestFormatPlaceholder(t *testing.T) {
	tests := []struct {
		alt, path string
		w, h      int
		want      string
	}{
		{"screenshot", "img.png", 800, 600, "[Image: screenshot - img.png (800x600)]"},
		{"", "photo.jpg", 1920, 1080, "[Image: image - photo.jpg (1920x1080)]"},
		{"logo", "logo.svg", 0, 0, "[Image: logo - logo.svg]"},
	}
	for _, tt := range tests {
		got := FormatPlaceholder(tt.alt, tt.path, tt.w, tt.h)
		if got != tt.want {
			t.Errorf("FormatPlaceholder(%q, %q, %d, %d) = %q, want %q",
				tt.alt, tt.path, tt.w, tt.h, got, tt.want)
		}
	}
}

func TestImageCache_Load(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.png")
	createTestPNG(t, imgPath, 100, 50)

	cache := NewImageCache()
	ci, err := cache.Load("test.png", dir, 200, GraphicsNone)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ci.OrigWidth != 100 || ci.OrigHeight != 50 {
		t.Errorf("orig dims = %dx%d, want 100x50", ci.OrigWidth, ci.OrigHeight)
	}
	// Image is smaller than max, should not be resized.
	if ci.Width != 100 || ci.Height != 50 {
		t.Errorf("resized dims = %dx%d, want 100x50", ci.Width, ci.Height)
	}
	if ci.TermRows < 1 {
		t.Errorf("TermRows = %d, want >= 1", ci.TermRows)
	}
	// No encoding for GraphicsNone.
	if ci.Encoded != "" {
		t.Errorf("expected empty Encoded for GraphicsNone")
	}
}

func TestImageCache_Load_Resize(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "wide.png")
	createTestPNG(t, imgPath, 400, 200)

	cache := NewImageCache()
	ci, err := cache.Load("wide.png", dir, 100, GraphicsNone)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ci.Width != 100 {
		t.Errorf("Width = %d, want 100", ci.Width)
	}
	if ci.Height != 50 {
		t.Errorf("Height = %d, want 50 (aspect ratio preserved)", ci.Height)
	}
}

func TestImageCache_Load_Cached(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "cached.png")
	createTestPNG(t, imgPath, 64, 32)

	cache := NewImageCache()
	ci1, err := cache.Load("cached.png", dir, 200, GraphicsNone)
	if err != nil {
		t.Fatal(err)
	}
	ci2, err := cache.Load("cached.png", dir, 200, GraphicsNone)
	if err != nil {
		t.Fatal(err)
	}
	if ci1 != ci2 {
		t.Error("expected same pointer from cache on second load")
	}
}

func TestImageCache_Load_InvalidatesOnResize(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "resize.png")
	createTestPNG(t, imgPath, 200, 100)

	cache := NewImageCache()
	ci1, _ := cache.Load("resize.png", dir, 300, GraphicsNone)
	ci2, _ := cache.Load("resize.png", dir, 100, GraphicsNone)
	if ci1 == ci2 {
		t.Error("expected different entries when maxWidth changes")
	}
	if ci2.Width != 100 {
		t.Errorf("Width = %d, want 100", ci2.Width)
	}
}

func TestImageCache_Load_NotFound(t *testing.T) {
	cache := NewImageCache()
	_, err := cache.Load("nonexistent.png", "/tmp", 200, GraphicsNone)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestImageCache_Clear(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "clear.png")
	createTestPNG(t, imgPath, 32, 32)

	cache := NewImageCache()
	ci1, _ := cache.Load("clear.png", dir, 200, GraphicsNone)
	cache.Clear()
	ci2, _ := cache.Load("clear.png", dir, 200, GraphicsNone)
	if ci1 == ci2 {
		t.Error("expected different entries after Clear")
	}
}

func TestImageCache_Load_Sixel(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "sixel.png")
	createTestPNG(t, imgPath, 8, 8)

	cache := NewImageCache()
	ci, err := cache.Load("sixel.png", dir, 200, GraphicsSixel)
	if err != nil {
		t.Fatalf("Load with Sixel failed: %v", err)
	}
	if ci.Encoded == "" {
		t.Error("expected non-empty Sixel encoding")
	}
	// Sixel starts with ESC P q
	if len(ci.Encoded) < 3 || ci.Encoded[:3] != "\x1bPq" {
		t.Errorf("Sixel encoding should start with ESC P q, got %q", ci.Encoded[:min(10, len(ci.Encoded))])
	}
}

func TestImageCache_Load_Kitty(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "kitty.png")
	createTestPNG(t, imgPath, 8, 8)

	cache := NewImageCache()
	ci, err := cache.Load("kitty.png", dir, 200, GraphicsKitty)
	if err != nil {
		t.Fatalf("Load with Kitty failed: %v", err)
	}
	if ci.Encoded == "" {
		t.Error("expected non-empty Kitty encoding")
	}
	// Kitty starts with ESC _ G
	if len(ci.Encoded) < 3 || ci.Encoded[:3] != "\x1b_G" {
		t.Errorf("Kitty encoding should start with ESC _ G, got %q", ci.Encoded[:min(10, len(ci.Encoded))])
	}
}

func TestCellHeight(t *testing.T) {
	h := CellHeight()
	if h != 16 {
		t.Errorf("CellHeight() = %d, want 16", h)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
