package editor

import (
	"testing"

	"numentext/internal/graphics"
)

func TestPendingImage_QueueAndClear(t *testing.T) {
	e := NewEditor()
	defer e.ClearPendingImages()

	if e.PendingImageCount() != 0 {
		t.Fatalf("expected 0 pending images, got %d", e.PendingImageCount())
	}

	e.pendingImages = append(e.pendingImages, PendingImage{
		ScreenRow:   5,
		ScreenCol:   10,
		Width:       40,
		Height:      3,
		EncodedData: "\x1bPq#0;2;100;0;0??\x1b\\",
		Protocol:    graphics.GraphicsSixel,
	})

	if e.PendingImageCount() != 1 {
		t.Fatalf("expected 1 pending image, got %d", e.PendingImageCount())
	}

	imgs := e.PendingImages()
	if len(imgs) != 1 {
		t.Fatalf("PendingImages returned %d items, want 1", len(imgs))
	}
	if imgs[0].ScreenRow != 5 || imgs[0].ScreenCol != 10 {
		t.Errorf("image position = (%d, %d), want (5, 10)", imgs[0].ScreenRow, imgs[0].ScreenCol)
	}
	if imgs[0].Protocol != graphics.GraphicsSixel {
		t.Errorf("protocol = %v, want Sixel", imgs[0].Protocol)
	}

	e.ClearPendingImages()
	if e.PendingImageCount() != 0 {
		t.Errorf("after clear: expected 0 pending images, got %d", e.PendingImageCount())
	}
}

func TestFlushPendingImages_NoGraphics(t *testing.T) {
	e := NewEditor()
	e.graphicsCap = graphics.GraphicsNone

	// Queue an image; flush should discard it without error.
	e.pendingImages = append(e.pendingImages, PendingImage{
		ScreenRow:   0,
		ScreenCol:   0,
		Width:       10,
		Height:      1,
		EncodedData: "test-data",
		Protocol:    graphics.GraphicsSixel,
	})
	e.flushPendingImages()

	if e.PendingImageCount() != 0 {
		t.Errorf("expected flush to clear queue, got %d", e.PendingImageCount())
	}
}

func TestFlushPendingImages_EmptyQueue(t *testing.T) {
	e := NewEditor()
	e.graphicsCap = graphics.GraphicsSixel

	// Should not panic with empty queue.
	e.flushPendingImages()
	if e.PendingImageCount() != 0 {
		t.Errorf("expected 0 pending images, got %d", e.PendingImageCount())
	}
}

func TestFlushPendingImages_SkipsEmptyEncoded(t *testing.T) {
	e := NewEditor()
	e.graphicsCap = graphics.GraphicsSixel

	// Image with empty EncodedData should be silently skipped.
	e.pendingImages = append(e.pendingImages, PendingImage{
		ScreenRow:   0,
		ScreenCol:   0,
		Width:       10,
		Height:      1,
		EncodedData: "",
		Protocol:    graphics.GraphicsSixel,
	})
	e.flushPendingImages()

	if e.PendingImageCount() != 0 {
		t.Errorf("expected flush to clear queue, got %d", e.PendingImageCount())
	}
}

func TestSetGraphicsCap(t *testing.T) {
	e := NewEditor()

	e.SetGraphicsCap(graphics.GraphicsSixel)
	if e.GraphicsCap() != graphics.GraphicsSixel {
		t.Errorf("expected Sixel, got %v", e.GraphicsCap())
	}

	e.SetGraphicsCap(graphics.GraphicsKitty)
	if e.GraphicsCap() != graphics.GraphicsKitty {
		t.Errorf("expected Kitty, got %v", e.GraphicsCap())
	}

	e.SetGraphicsCap(graphics.GraphicsNone)
	if e.GraphicsCap() != graphics.GraphicsNone {
		t.Errorf("expected None, got %v", e.GraphicsCap())
	}
}

func TestEstimateCellHeight(t *testing.T) {
	h := EstimateCellHeight()
	if h <= 0 {
		t.Errorf("EstimateCellHeight() = %d, want > 0", h)
	}
	if h != 16 {
		t.Errorf("EstimateCellHeight() = %d, want 16 (default)", h)
	}
}

func TestPendingImage_MultipleImages(t *testing.T) {
	e := NewEditor()
	defer e.ClearPendingImages()

	for i := 0; i < 5; i++ {
		e.pendingImages = append(e.pendingImages, PendingImage{
			ScreenRow:   i * 3,
			ScreenCol:   0,
			Width:       80,
			Height:      3,
			EncodedData: "\x1bPq??\x1b\\",
			Protocol:    graphics.GraphicsSixel,
		})
	}

	if e.PendingImageCount() != 5 {
		t.Fatalf("expected 5 pending images, got %d", e.PendingImageCount())
	}

	imgs := e.PendingImages()
	for i, img := range imgs {
		if img.ScreenRow != i*3 {
			t.Errorf("image[%d].ScreenRow = %d, want %d", i, img.ScreenRow, i*3)
		}
	}
}

func TestPendingImage_KittyProtocol(t *testing.T) {
	e := NewEditor()
	defer e.ClearPendingImages()

	e.pendingImages = append(e.pendingImages, PendingImage{
		ScreenRow:   2,
		ScreenCol:   5,
		Width:       30,
		Height:      4,
		EncodedData: "\x1b_Gf=100,t=d,s=8,v=8,m=0;data\x1b\\",
		Protocol:    graphics.GraphicsKitty,
	})

	imgs := e.PendingImages()
	if len(imgs) != 1 {
		t.Fatalf("expected 1 image, got %d", len(imgs))
	}
	if imgs[0].Protocol != graphics.GraphicsKitty {
		t.Errorf("protocol = %v, want Kitty", imgs[0].Protocol)
	}
}

func TestFloatImageState_StartAndEnd(t *testing.T) {
	e := NewEditor()

	// Initially no float is active.
	if e.FloatImageCols() != 0 {
		t.Errorf("initial FloatImageCols = %d, want 0", e.FloatImageCols())
	}
	if e.FloatImageRows() != 0 {
		t.Errorf("initial FloatImageRows = %d, want 0", e.FloatImageRows())
	}

	// Set a floating image: 20 cols wide, 5 rows tall, anchored at line 3.
	e.SetFloatImage(20, 5, 3)
	if e.FloatImageCols() != 20 {
		t.Errorf("FloatImageCols = %d, want 20", e.FloatImageCols())
	}
	if e.FloatImageRows() != 5 {
		t.Errorf("FloatImageRows = %d, want 5", e.FloatImageRows())
	}
	if e.FloatImageLineIdx() != 3 {
		t.Errorf("FloatImageLineIdx = %d, want 3", e.FloatImageLineIdx())
	}

	// Clear float state.
	e.ClearFloatImage()
	if e.FloatImageCols() != 0 {
		t.Errorf("after clear FloatImageCols = %d, want 0", e.FloatImageCols())
	}
	if e.FloatImageRows() != 0 {
		t.Errorf("after clear FloatImageRows = %d, want 0", e.FloatImageRows())
	}
}

func TestFloatImageState_DecrementToZero(t *testing.T) {
	e := NewEditor()

	// Simulate float with 3 remaining rows.
	e.SetFloatImage(15, 3, 0)

	// Simulate decrementing rows as lines are drawn.
	for i := 3; i > 0; i-- {
		if e.FloatImageRows() != i {
			t.Errorf("iteration %d: FloatImageRows = %d, want %d", 3-i, e.FloatImageRows(), i)
		}
		if e.FloatImageCols() != 15 {
			t.Errorf("iteration %d: FloatImageCols = %d, want 15", 3-i, e.FloatImageCols())
		}
		// Decrement (same logic as draw loop).
		e.SetFloatImage(e.FloatImageCols(), e.FloatImageRows()-1, e.FloatImageLineIdx())
		if e.FloatImageRows() <= 0 {
			e.ClearFloatImage()
		}
	}

	// After 3 decrements, float should be cleared.
	if e.FloatImageCols() != 0 {
		t.Errorf("after exhaust: FloatImageCols = %d, want 0", e.FloatImageCols())
	}
	if e.FloatImageRows() != 0 {
		t.Errorf("after exhaust: FloatImageRows = %d, want 0", e.FloatImageRows())
	}
}

func TestFloatImageState_EditorXShift(t *testing.T) {
	// Verify the expected shift calculation: floatImageCols + 1 (gap).
	e := NewEditor()
	e.SetFloatImage(25, 4, 0)

	imgCols := e.FloatImageCols()
	expectedShift := imgCols + 1 // image width + gap
	if expectedShift != 26 {
		t.Errorf("expected shift = %d, want 26", expectedShift)
	}

	// For an 80-col editor, text width should be 80 - 26 = 54
	editorWidth := 80
	textWidth := editorWidth - expectedShift
	if textWidth != 54 {
		t.Errorf("text width = %d, want 54", textWidth)
	}
}

func TestFloatImageState_NewImageEndsOldFloat(t *testing.T) {
	e := NewEditor()

	// Start first float.
	e.SetFloatImage(20, 10, 5)

	// Simulate a new image starting while old float is active.
	// The draw loop resets float state before setting new image.
	if e.FloatImageRows() > 0 {
		e.ClearFloatImage()
	}

	// Set new float.
	e.SetFloatImage(15, 6, 12)
	if e.FloatImageCols() != 15 {
		t.Errorf("new float cols = %d, want 15", e.FloatImageCols())
	}
	if e.FloatImageRows() != 6 {
		t.Errorf("new float rows = %d, want 6", e.FloatImageRows())
	}
	if e.FloatImageLineIdx() != 12 {
		t.Errorf("new float lineIdx = %d, want 12", e.FloatImageLineIdx())
	}
}
