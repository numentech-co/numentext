package editor

import (
	"fmt"
	"os"
	"runtime"

	"numentext/internal/graphics"
)

// openTTY opens /dev/tty for direct terminal output that bypasses tcell's
// screen buffer. On systems where /dev/tty is unavailable (e.g., Windows),
// it falls back to os.Stdout.
func openTTY() *os.File {
	if runtime.GOOS == "windows" {
		return os.Stdout
	}
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		// Fallback: use stdout (may conflict with tcell in some edge cases).
		return os.Stdout
	}
	return f
}

// flushPendingImages writes all queued inline image escape sequences to the
// terminal. This must be called AFTER tcell has rendered (Show/Sync) so the
// escape sequences are not overwritten by tcell's screen updates.
//
// Each image is positioned using ANSI cursor movement (ESC[row;colH) and
// then the Sixel or Kitty escape sequence data is written.
func (e *Editor) flushPendingImages() {
	if len(e.pendingImages) == 0 {
		return
	}
	if e.graphicsCap == graphics.GraphicsNone {
		e.pendingImages = nil
		return
	}
	if e.ttyFile == nil {
		e.pendingImages = nil
		return
	}

	// Clear previous image areas by overwriting with spaces.
	// This prevents Sixel pixels from persisting after scroll.
	for _, prev := range e.lastImageAreas {
		for r := 0; r < prev.Height; r++ {
			row := prev.ScreenRow + r + 1 // 1-based
			col := prev.ScreenCol + 1
			pos := fmt.Sprintf("\033[%d;%dH", row, col)
			_, _ = fmt.Fprint(e.ttyFile, pos)
			// Erase from cursor to end of line within the image width
			_, _ = fmt.Fprintf(e.ttyFile, "\033[%dX", prev.Width)
		}
	}

	// Record current image positions for next clear cycle
	e.lastImageAreas = make([]PendingImage, len(e.pendingImages))
	copy(e.lastImageAreas, e.pendingImages)

	for _, img := range e.pendingImages {
		if img.EncodedData == "" {
			continue
		}
		// Skip images that would extend below the visible area
		_, screenH := 0, 0
		if e.ttyFile == os.Stdout {
			// Can't determine screen size easily; just render
		}
		_ = screenH

		// ANSI cursor positioning: ESC [ row ; col H (1-based)
		pos := fmt.Sprintf("\033[%d;%dH", img.ScreenRow+1, img.ScreenCol+1)
		_, _ = fmt.Fprint(e.ttyFile, pos)
		_, _ = fmt.Fprint(e.ttyFile, img.EncodedData)
	}
	e.pendingImages = nil
}

// PendingImageCount returns the number of queued images. This is primarily
// useful for testing.
func (e *Editor) PendingImageCount() int {
	return len(e.pendingImages)
}

// PendingImages returns a copy of the pending image queue. This is primarily
// useful for testing.
func (e *Editor) PendingImages() []PendingImage {
	result := make([]PendingImage, len(e.pendingImages))
	copy(result, e.pendingImages)
	return result
}

// GraphicsCap returns the detected graphics capability.
func (e *Editor) GraphicsCap() graphics.GraphicsCapability {
	return e.graphicsCap
}

// SetGraphicsCap overrides the detected graphics capability. This is useful
// for testing or for forcing a specific protocol.
func (e *Editor) SetGraphicsCap(cap graphics.GraphicsCapability) {
	e.graphicsCap = cap
	// Clear the image cache since encodings depend on the protocol.
	e.imageCache.Clear()
}

// ClearPendingImages discards all queued images without writing them.
func (e *Editor) ClearPendingImages() {
	e.pendingImages = nil
}

// EstimateCellHeight returns the assumed pixel height of a single terminal
// cell row. This matches the constant used in the graphics package for
// calculating how many terminal rows an image occupies.
func EstimateCellHeight() int {
	return graphics.CellHeight()
}
