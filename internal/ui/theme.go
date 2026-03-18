package ui

import "github.com/gdamore/tcell/v2"

// Borland-style color scheme
var (
	// Main backgrounds
	ColorBg        = tcell.NewRGBColor(0, 0, 128)     // Dark blue #000080
	ColorBgAlt  = tcell.NewRGBColor(0, 0, 100)     // Darker blue
	ColorMenuBg    = tcell.NewRGBColor(0, 170, 170)    // Cyan
	ColorStatusBg  = tcell.NewRGBColor(0, 170, 170)    // Cyan
	ColorOutputBg    = tcell.NewRGBColor(0, 0, 80)      // Very dark blue
	ColorOutputBgStripe = tcell.NewRGBColor(0, 0, 60)     // Slightly darker blue for alternating blocks
	ColorDialogBg  = tcell.NewRGBColor(0, 170, 170)    // Cyan for dialogs
	ColorGutterBg  = tcell.NewRGBColor(0, 0, 100)      // Darker blue for gutter

	// Text colors
	ColorText       = tcell.NewRGBColor(255, 255, 85)  // Yellow
	ColorTextPrimary  = tcell.NewRGBColor(255, 255, 255) // White
	ColorTextMuted   = tcell.NewRGBColor(170, 170, 170) // Gray
	ColorMenuText   = tcell.NewRGBColor(255, 255, 255) // White
	ColorStatusText = tcell.NewRGBColor(0, 0, 0)       // Black
	ColorGutterText = tcell.NewRGBColor(170, 170, 170) // Gray

	// Syntax highlighting
	ColorKeyword = tcell.NewRGBColor(255, 255, 255) // White (bold)
	ColorString  = tcell.NewRGBColor(0, 255, 255)   // Cyan
	ColorComment = tcell.NewRGBColor(170, 170, 170) // Gray
	ColorNumber  = tcell.NewRGBColor(255, 85, 255)  // Magenta
	ColorType    = tcell.NewRGBColor(85, 255, 85)   // Green
	ColorFunc    = tcell.NewRGBColor(255, 255, 255)  // White

	// UI elements
	ColorTabActive   = tcell.NewRGBColor(255, 255, 255) // White text
	ColorTabInactive = tcell.NewRGBColor(170, 170, 170) // Gray text
	ColorTabActiveBg = tcell.NewRGBColor(0, 0, 128)     // Blue
	ColorTabBarBg    = tcell.NewRGBColor(0, 0, 80)      // Darker blue
	ColorSelected    = tcell.NewRGBColor(0, 170, 170)    // Cyan bg for selection
	ColorSelectedText = tcell.NewRGBColor(0, 0, 0)       // Black text on selection

	// File tree
	ColorTreeText     = tcell.NewRGBColor(255, 255, 255)
	ColorTreeSelected = tcell.NewRGBColor(0, 170, 170)

	// Menu dropdown
	ColorMenuDropBg    = tcell.NewRGBColor(0, 170, 170)
	ColorMenuDropText  = tcell.NewRGBColor(0, 0, 0)
	ColorMenuHighlight = tcell.NewRGBColor(0, 0, 128)
	ColorMenuHlText    = tcell.NewRGBColor(255, 255, 255)
	ColorMenuShortcut  = tcell.NewRGBColor(85, 85, 85)

	// Accelerator highlight
	ColorAccel = tcell.NewRGBColor(255, 85, 85) // Red/bright for accelerator letters

	// Border
	ColorBorder = tcell.NewRGBColor(0, 170, 170)

	// Bracket matching
	ColorBracketMatch   = tcell.NewRGBColor(0, 170, 0)    // Green background for matching brackets
	ColorBracketMatchFg = tcell.NewRGBColor(255, 255, 255) // White text on match
	ColorBracketError   = tcell.NewRGBColor(170, 0, 0)     // Red background for unmatched brackets
	ColorBracketErrorFg = tcell.NewRGBColor(255, 255, 255)  // White text on error

	// Panel focus indicator
	ColorPanelFocused = tcell.NewRGBColor(255, 255, 85)  // Yellow - active panel border/title
	ColorPanelBlurred = tcell.NewRGBColor(85, 85, 85)    // Dark gray - inactive panel title

	// Markdown preview colors
	ColorMarkdownH1     = tcell.NewRGBColor(255, 255, 85)  // Yellow - level 1 headers
	ColorMarkdownH2     = tcell.NewRGBColor(85, 255, 255)  // Cyan - level 2 headers
	ColorMarkdownH3     = tcell.NewRGBColor(85, 255, 85)   // Green - level 3 headers
	ColorMarkdownH4     = tcell.NewRGBColor(255, 170, 85)  // Orange - level 4 headers
	ColorMarkdownH5     = tcell.NewRGBColor(170, 170, 255) // Light blue - level 5 headers
	ColorMarkdownH6     = tcell.NewRGBColor(170, 170, 170) // Gray - level 6 headers
	ColorMarkdownLink   = tcell.NewRGBColor(0, 255, 255)   // Cyan - links
	ColorMarkdownCode   = tcell.NewRGBColor(255, 170, 85)  // Orange - code spans
	ColorMarkdownCodeBg = tcell.NewRGBColor(0, 0, 80)      // Dark blue - code background
)

// ThemeStyle returns a tcell.Style for the given role
func ThemeStyle(role string) tcell.Style {
	base := tcell.StyleDefault
	switch role {
	case "editor":
		return base.Foreground(ColorText).Background(ColorBg)
	case "menu":
		return base.Foreground(ColorMenuText).Background(ColorMenuBg)
	case "status":
		return base.Foreground(ColorStatusText).Background(ColorStatusBg)
	case "output":
		return base.Foreground(ColorTextPrimary).Background(ColorOutputBg)
	case "gutter":
		return base.Foreground(ColorGutterText).Background(ColorGutterBg)
	case "dialog":
		return base.Foreground(ColorStatusText).Background(ColorDialogBg)
	default:
		return base.Foreground(ColorTextPrimary).Background(ColorBg)
	}
}
