package graphics

import (
	"os"
	"testing"
)

func TestDetectCapability_Kitty(t *testing.T) {
	// Save and restore env.
	origKitty := os.Getenv("KITTY_WINDOW_ID")
	origTerm := os.Getenv("TERM_PROGRAM")
	defer func() {
		os.Setenv("KITTY_WINDOW_ID", origKitty)
		os.Setenv("TERM_PROGRAM", origTerm)
	}()

	os.Setenv("KITTY_WINDOW_ID", "12345")
	os.Setenv("TERM_PROGRAM", "")

	cap := DetectCapability()
	if cap != GraphicsKitty {
		t.Errorf("expected GraphicsKitty, got %v", cap)
	}
}

func TestDetectCapability_Sixel_iTerm(t *testing.T) {
	origKitty := os.Getenv("KITTY_WINDOW_ID")
	origTerm := os.Getenv("TERM_PROGRAM")
	defer func() {
		os.Setenv("KITTY_WINDOW_ID", origKitty)
		os.Setenv("TERM_PROGRAM", origTerm)
	}()

	os.Unsetenv("KITTY_WINDOW_ID")
	os.Setenv("TERM_PROGRAM", "iTerm.app")

	cap := DetectCapability()
	if cap != GraphicsSixel {
		t.Errorf("expected GraphicsSixel, got %v", cap)
	}
}

func TestDetectCapability_Sixel_WezTerm(t *testing.T) {
	origKitty := os.Getenv("KITTY_WINDOW_ID")
	origTerm := os.Getenv("TERM_PROGRAM")
	defer func() {
		os.Setenv("KITTY_WINDOW_ID", origKitty)
		os.Setenv("TERM_PROGRAM", origTerm)
	}()

	os.Unsetenv("KITTY_WINDOW_ID")
	os.Setenv("TERM_PROGRAM", "WezTerm")

	cap := DetectCapability()
	if cap != GraphicsSixel {
		t.Errorf("expected GraphicsSixel, got %v", cap)
	}
}

func TestDetectCapability_None(t *testing.T) {
	origKitty := os.Getenv("KITTY_WINDOW_ID")
	origTerm := os.Getenv("TERM_PROGRAM")
	defer func() {
		os.Setenv("KITTY_WINDOW_ID", origKitty)
		os.Setenv("TERM_PROGRAM", origTerm)
	}()

	os.Unsetenv("KITTY_WINDOW_ID")
	os.Setenv("TERM_PROGRAM", "Apple_Terminal")

	cap := DetectCapability()
	if cap != GraphicsNone {
		t.Errorf("expected GraphicsNone, got %v", cap)
	}
}

func TestDetectCapability_KittyTakesPriority(t *testing.T) {
	origKitty := os.Getenv("KITTY_WINDOW_ID")
	origTerm := os.Getenv("TERM_PROGRAM")
	defer func() {
		os.Setenv("KITTY_WINDOW_ID", origKitty)
		os.Setenv("TERM_PROGRAM", origTerm)
	}()

	// Both set: Kitty should win.
	os.Setenv("KITTY_WINDOW_ID", "1")
	os.Setenv("TERM_PROGRAM", "iTerm.app")

	cap := DetectCapability()
	if cap != GraphicsKitty {
		t.Errorf("expected GraphicsKitty when both set, got %v", cap)
	}
}

func TestGraphicsCapability_String(t *testing.T) {
	tests := []struct {
		cap  GraphicsCapability
		want string
	}{
		{GraphicsNone, "None"},
		{GraphicsSixel, "Sixel"},
		{GraphicsKitty, "Kitty"},
	}
	for _, tt := range tests {
		if got := tt.cap.String(); got != tt.want {
			t.Errorf("%d.String() = %q, want %q", tt.cap, got, tt.want)
		}
	}
}
