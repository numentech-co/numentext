package graphics

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/draw"
)

// CachedImage holds a loaded, resized, and encoded image ready for rendering.
type CachedImage struct {
	// OrigWidth and OrigHeight are the original image dimensions in pixels.
	OrigWidth  int
	OrigHeight int

	// Width and Height are the resized dimensions in pixels.
	Width  int
	Height int

	// TermRows is the number of terminal rows the image occupies,
	// calculated as ceil(Height / cellHeight).
	TermRows int

	// Encoded holds the terminal escape sequence (Sixel or Kitty) for
	// rendering. Empty when capability is GraphicsNone.
	Encoded string

	// TermWidth is the terminal column width the image was encoded for.
	// Used to invalidate the cache when the terminal is resized.
	TermWidth int

	// Cap records which protocol the image was encoded for.
	Cap GraphicsCapability
}

// ImageCache provides thread-safe caching of loaded and encoded images.
// The cache key is the absolute file path.
type ImageCache struct {
	mu    sync.Mutex
	cache map[string]*CachedImage
}

// NewImageCache creates an empty image cache.
func NewImageCache() *ImageCache {
	return &ImageCache{
		cache: make(map[string]*CachedImage),
	}
}

// cellHeight is the assumed pixel height of a single terminal cell row.
// Most terminals default to roughly 16-20px per row; 16 is a safe default.
const cellHeight = 16

// CellHeight returns the assumed pixel height of a single terminal cell row.
func CellHeight() int {
	return cellHeight
}

// Load loads an image from disk, resizes it to fit maxWidthPx pixels wide
// (preserving aspect ratio), encodes it for the given capability, and
// caches the result.  Subsequent calls with the same path and maxWidthPx
// return the cached version.
//
// basePath is the directory used to resolve relative image paths (typically
// the directory containing the markdown file).
func (ic *ImageCache) Load(path string, basePath string, maxWidthPx int, cap GraphicsCapability) (*CachedImage, error) {
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(basePath, path)
	}
	absPath = filepath.Clean(absPath)

	ic.mu.Lock()
	defer ic.mu.Unlock()

	// Return cached entry if dimensions match.
	if cached, ok := ic.cache[absPath]; ok && cached.TermWidth == maxWidthPx && cached.Cap == cap {
		return cached, nil
	}

	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("open image %s: %w", absPath, err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image %s: %w", absPath, err)
	}

	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	// Resize to fit maxWidthPx, preserving aspect ratio.
	newW := origW
	newH := origH
	if newW > maxWidthPx && maxWidthPx > 0 {
		newH = origH * maxWidthPx / origW
		newW = maxWidthPx
	}
	if newW <= 0 {
		newW = 1
	}
	if newH <= 0 {
		newH = 1
	}

	// Resize the image using CatmullRom (high quality).
	resized := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(resized, resized.Bounds(), img, bounds, draw.Over, nil)

	// Calculate terminal rows.
	termRows := (newH + cellHeight - 1) / cellHeight
	if termRows < 1 {
		termRows = 1
	}

	// Encode for the target protocol.
	var encoded string
	switch cap {
	case GraphicsSixel:
		encoded = EncodeSixel(resized)
	case GraphicsKitty:
		encoded, err = EncodeKitty(resized)
		if err != nil {
			return nil, fmt.Errorf("kitty encode %s: %w", absPath, err)
		}
	default:
		// No encoding needed for placeholder mode.
	}

	ci := &CachedImage{
		OrigWidth:  origW,
		OrigHeight: origH,
		Width:      newW,
		Height:     newH,
		TermRows:   termRows,
		Encoded:    encoded,
		TermWidth:  maxWidthPx,
		Cap:        cap,
	}
	ic.cache[absPath] = ci
	return ci, nil
}

// Invalidate removes a specific path from the cache.
func (ic *ImageCache) Invalidate(path string) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	delete(ic.cache, path)
}

// Clear removes all entries from the cache.
func (ic *ImageCache) Clear() {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	ic.cache = make(map[string]*CachedImage)
}

// ResolvePath resolves an image path relative to basePath and returns the
// absolute path. This is useful for cache key lookups.
func ResolvePath(path string, basePath string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(basePath, path))
}

// FormatPlaceholder returns a text placeholder string for terminals that
// do not support inline images. Format: [Image: alt - path (WxH)]
// If the image cannot be loaded, dimensions are omitted.
func FormatPlaceholder(alt, path string, width, height int) string {
	dims := ""
	if width > 0 && height > 0 {
		dims = fmt.Sprintf(" (%dx%d)", width, height)
	}
	if alt == "" {
		alt = "image"
	}
	return fmt.Sprintf("[Image: %s - %s%s]", alt, path, dims)
}
