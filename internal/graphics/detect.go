package graphics

import "os"

// GraphicsCapability represents the terminal's inline image rendering support.
type GraphicsCapability int

const (
	// GraphicsNone means no inline image protocol is available.
	GraphicsNone GraphicsCapability = iota
	// GraphicsSixel means the terminal supports Sixel graphics.
	GraphicsSixel
	// GraphicsKitty means the terminal supports the Kitty graphics protocol.
	GraphicsKitty
	// GraphicsITerm means the terminal supports iTerm2's inline image protocol.
	GraphicsITerm
)

// String returns a human-readable name for the capability.
func (g GraphicsCapability) String() string {
	switch g {
	case GraphicsSixel:
		return "Sixel"
	case GraphicsKitty:
		return "Kitty"
	case GraphicsITerm:
		return "iTerm"
	default:
		return "None"
	}
}

// DetectCapability checks environment variables and terminal identifiers
// to determine which inline image protocol (if any) the terminal supports.
//
// Detection order:
//  1. KITTY_WINDOW_ID set -> Kitty protocol
//  2. TERM_PROGRAM is iTerm.app -> iTerm2 inline image protocol
//  3. TERM_PROGRAM is WezTerm -> Sixel
//  4. Otherwise -> None
func DetectCapability() GraphicsCapability {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return GraphicsKitty
	}
	term := os.Getenv("TERM_PROGRAM")
	switch term {
	case "iTerm.app":
		return GraphicsITerm
	case "WezTerm":
		return GraphicsSixel
	}
	return GraphicsNone
}
