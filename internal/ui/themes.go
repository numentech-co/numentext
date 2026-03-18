package ui

import "github.com/gdamore/tcell/v2"

// Theme defines a complete color palette for the UI.
type Theme struct {
	Name string

	// Main backgrounds
	Bg, BgAlt, MenuBg, StatusBg, OutputBg, OutputBgStripe, DialogBg, GutterBg tcell.Color

	// Text colors
	Text, TextPrimary, TextMuted, MenuText, StatusText, GutterText tcell.Color

	// Syntax highlighting
	Keyword, String, Comment, Number, Type, Func tcell.Color

	// UI elements
	TabActive, TabInactive tcell.Color
	TabActiveBg, TabBarBg  tcell.Color
	Selected, SelectedText tcell.Color

	// File tree
	TreeText, TreeSelected tcell.Color

	// Menu dropdown
	MenuDropBg, MenuDropText, MenuHighlight, MenuHlText, MenuShortcut tcell.Color

	// Accents
	Accel, Border, PanelFocused, PanelBlurred tcell.Color
}

// Built-in themes
var themes = map[string]*Theme{
	"borland": {
		Name:         "borland",
		Bg:           tcell.NewRGBColor(0, 0, 128),
		BgAlt:     tcell.NewRGBColor(0, 0, 100),
		MenuBg:       tcell.NewRGBColor(0, 170, 170),
		StatusBg:     tcell.NewRGBColor(0, 170, 170),
		OutputBg:     tcell.NewRGBColor(0, 0, 80),
		OutputBgStripe:  tcell.NewRGBColor(0, 0, 60),
		DialogBg:     tcell.NewRGBColor(0, 170, 170),
		GutterBg:     tcell.NewRGBColor(0, 0, 100),
		Text:         tcell.NewRGBColor(255, 255, 85),
		TextPrimary:    tcell.NewRGBColor(255, 255, 255),
		TextMuted:     tcell.NewRGBColor(170, 170, 170),
		MenuText:     tcell.NewRGBColor(255, 255, 255),
		StatusText:   tcell.NewRGBColor(0, 0, 0),
		GutterText:   tcell.NewRGBColor(170, 170, 170),
		Keyword:      tcell.NewRGBColor(255, 255, 255),
		String:       tcell.NewRGBColor(0, 255, 255),
		Comment:      tcell.NewRGBColor(170, 170, 170),
		Number:       tcell.NewRGBColor(255, 85, 255),
		Type:         tcell.NewRGBColor(85, 255, 85),
		Func:         tcell.NewRGBColor(255, 255, 255),
		TabActive:    tcell.NewRGBColor(255, 255, 255),
		TabInactive:  tcell.NewRGBColor(170, 170, 170),
		TabActiveBg:  tcell.NewRGBColor(0, 0, 128),
		TabBarBg:     tcell.NewRGBColor(0, 0, 80),
		Selected:     tcell.NewRGBColor(0, 170, 170),
		SelectedText: tcell.NewRGBColor(0, 0, 0),
		TreeText:     tcell.NewRGBColor(255, 255, 255),
		TreeSelected: tcell.NewRGBColor(0, 170, 170),
		MenuDropBg:   tcell.NewRGBColor(0, 170, 170),
		MenuDropText: tcell.NewRGBColor(0, 0, 0),
		MenuHighlight: tcell.NewRGBColor(0, 0, 128),
		MenuHlText:   tcell.NewRGBColor(255, 255, 255),
		MenuShortcut: tcell.NewRGBColor(85, 85, 85),
		Accel:        tcell.NewRGBColor(255, 85, 85),
		Border:       tcell.NewRGBColor(0, 170, 170),
		PanelFocused: tcell.NewRGBColor(255, 255, 85),
		PanelBlurred: tcell.NewRGBColor(85, 85, 85),
	},
	"modern-dark": {
		Name:         "modern-dark",
		Bg:           tcell.NewRGBColor(30, 30, 30),
		BgAlt:     tcell.NewRGBColor(20, 20, 20),
		MenuBg:       tcell.NewRGBColor(50, 50, 50),
		StatusBg:     tcell.NewRGBColor(0, 122, 204),
		OutputBg:     tcell.NewRGBColor(25, 25, 25),
		OutputBgStripe:  tcell.NewRGBColor(35, 35, 35),
		DialogBg:     tcell.NewRGBColor(60, 60, 60),
		GutterBg:     tcell.NewRGBColor(30, 30, 30),
		Text:         tcell.NewRGBColor(212, 212, 212),
		TextPrimary:    tcell.NewRGBColor(255, 255, 255),
		TextMuted:     tcell.NewRGBColor(128, 128, 128),
		MenuText:     tcell.NewRGBColor(204, 204, 204),
		StatusText:   tcell.NewRGBColor(255, 255, 255),
		GutterText:   tcell.NewRGBColor(100, 100, 100),
		Keyword:      tcell.NewRGBColor(86, 156, 214),
		String:       tcell.NewRGBColor(206, 145, 120),
		Comment:      tcell.NewRGBColor(106, 153, 85),
		Number:       tcell.NewRGBColor(181, 206, 168),
		Type:         tcell.NewRGBColor(78, 201, 176),
		Func:         tcell.NewRGBColor(220, 220, 170),
		TabActive:    tcell.NewRGBColor(255, 255, 255),
		TabInactive:  tcell.NewRGBColor(128, 128, 128),
		TabActiveBg:  tcell.NewRGBColor(30, 30, 30),
		TabBarBg:     tcell.NewRGBColor(45, 45, 45),
		Selected:     tcell.NewRGBColor(38, 79, 120),
		SelectedText: tcell.NewRGBColor(255, 255, 255),
		TreeText:     tcell.NewRGBColor(204, 204, 204),
		TreeSelected: tcell.NewRGBColor(38, 79, 120),
		MenuDropBg:   tcell.NewRGBColor(60, 60, 60),
		MenuDropText: tcell.NewRGBColor(204, 204, 204),
		MenuHighlight: tcell.NewRGBColor(4, 57, 94),
		MenuHlText:   tcell.NewRGBColor(255, 255, 255),
		MenuShortcut: tcell.NewRGBColor(128, 128, 128),
		Accel:        tcell.NewRGBColor(0, 122, 204),
		Border:       tcell.NewRGBColor(60, 60, 60),
		PanelFocused: tcell.NewRGBColor(0, 122, 204),
		PanelBlurred: tcell.NewRGBColor(60, 60, 60),
	},
	"modern-light": {
		Name:         "modern-light",
		Bg:           tcell.NewRGBColor(255, 255, 255),
		BgAlt:     tcell.NewRGBColor(240, 240, 240),
		MenuBg:       tcell.NewRGBColor(221, 221, 221),
		StatusBg:     tcell.NewRGBColor(0, 122, 204),
		OutputBg:     tcell.NewRGBColor(245, 245, 245),
		OutputBgStripe:  tcell.NewRGBColor(235, 235, 235),
		DialogBg:     tcell.NewRGBColor(240, 240, 240),
		GutterBg:     tcell.NewRGBColor(245, 245, 245),
		Text:         tcell.NewRGBColor(0, 0, 0),
		TextPrimary:    tcell.NewRGBColor(0, 0, 0),
		TextMuted:     tcell.NewRGBColor(128, 128, 128),
		MenuText:     tcell.NewRGBColor(51, 51, 51),
		StatusText:   tcell.NewRGBColor(255, 255, 255),
		GutterText:   tcell.NewRGBColor(150, 150, 150),
		Keyword:      tcell.NewRGBColor(0, 0, 255),
		String:       tcell.NewRGBColor(163, 21, 21),
		Comment:      tcell.NewRGBColor(0, 128, 0),
		Number:       tcell.NewRGBColor(9, 134, 88),
		Type:         tcell.NewRGBColor(38, 127, 153),
		Func:         tcell.NewRGBColor(121, 94, 38),
		TabActive:    tcell.NewRGBColor(0, 0, 0),
		TabInactive:  tcell.NewRGBColor(128, 128, 128),
		TabActiveBg:  tcell.NewRGBColor(255, 255, 255),
		TabBarBg:     tcell.NewRGBColor(236, 236, 236),
		Selected:     tcell.NewRGBColor(173, 214, 255),
		SelectedText: tcell.NewRGBColor(0, 0, 0),
		TreeText:     tcell.NewRGBColor(51, 51, 51),
		TreeSelected: tcell.NewRGBColor(173, 214, 255),
		MenuDropBg:   tcell.NewRGBColor(240, 240, 240),
		MenuDropText: tcell.NewRGBColor(51, 51, 51),
		MenuHighlight: tcell.NewRGBColor(0, 122, 204),
		MenuHlText:   tcell.NewRGBColor(255, 255, 255),
		MenuShortcut: tcell.NewRGBColor(128, 128, 128),
		Accel:        tcell.NewRGBColor(0, 122, 204),
		Border:       tcell.NewRGBColor(200, 200, 200),
		PanelFocused: tcell.NewRGBColor(0, 122, 204),
		PanelBlurred: tcell.NewRGBColor(200, 200, 200),
	},
	"solarized-dark": {
		Name:         "solarized-dark",
		Bg:           tcell.NewRGBColor(0, 43, 54),
		BgAlt:     tcell.NewRGBColor(0, 34, 43),
		MenuBg:       tcell.NewRGBColor(7, 54, 66),
		StatusBg:     tcell.NewRGBColor(88, 110, 117),
		OutputBg:     tcell.NewRGBColor(0, 34, 43),
		OutputBgStripe:  tcell.NewRGBColor(7, 54, 66),
		DialogBg:     tcell.NewRGBColor(7, 54, 66),
		GutterBg:     tcell.NewRGBColor(0, 34, 43),
		Text:         tcell.NewRGBColor(131, 148, 150),
		TextPrimary:    tcell.NewRGBColor(253, 246, 227),
		TextMuted:     tcell.NewRGBColor(88, 110, 117),
		MenuText:     tcell.NewRGBColor(238, 232, 213),
		StatusText:   tcell.NewRGBColor(253, 246, 227),
		GutterText:   tcell.NewRGBColor(88, 110, 117),
		Keyword:      tcell.NewRGBColor(133, 153, 0),
		String:       tcell.NewRGBColor(42, 161, 152),
		Comment:      tcell.NewRGBColor(88, 110, 117),
		Number:       tcell.NewRGBColor(211, 54, 130),
		Type:         tcell.NewRGBColor(181, 137, 0),
		Func:         tcell.NewRGBColor(38, 139, 210),
		TabActive:    tcell.NewRGBColor(253, 246, 227),
		TabInactive:  tcell.NewRGBColor(88, 110, 117),
		TabActiveBg:  tcell.NewRGBColor(0, 43, 54),
		TabBarBg:     tcell.NewRGBColor(0, 34, 43),
		Selected:     tcell.NewRGBColor(7, 54, 66),
		SelectedText: tcell.NewRGBColor(253, 246, 227),
		TreeText:     tcell.NewRGBColor(238, 232, 213),
		TreeSelected: tcell.NewRGBColor(38, 139, 210),
		MenuDropBg:   tcell.NewRGBColor(7, 54, 66),
		MenuDropText: tcell.NewRGBColor(238, 232, 213),
		MenuHighlight: tcell.NewRGBColor(0, 43, 54),
		MenuHlText:   tcell.NewRGBColor(253, 246, 227),
		MenuShortcut: tcell.NewRGBColor(88, 110, 117),
		Accel:        tcell.NewRGBColor(203, 75, 22),
		Border:       tcell.NewRGBColor(88, 110, 117),
		PanelFocused: tcell.NewRGBColor(181, 137, 0),
		PanelBlurred: tcell.NewRGBColor(88, 110, 117),
	},
}

// ThemeNames returns the list of available theme names.
func ThemeNames() []string {
	return []string{"borland", "modern-dark", "modern-light", "solarized-dark"}
}

// ApplyTheme activates a named theme by reassigning all Color* variables.
func ApplyTheme(name string) {
	t, ok := themes[name]
	if !ok {
		return
	}

	ColorBg = t.Bg
	ColorBgAlt = t.BgAlt
	ColorMenuBg = t.MenuBg
	ColorStatusBg = t.StatusBg
	ColorOutputBg = t.OutputBg
	ColorOutputBgStripe = t.OutputBgStripe
	ColorDialogBg = t.DialogBg
	ColorGutterBg = t.GutterBg

	ColorText = t.Text
	ColorTextPrimary = t.TextPrimary
	ColorTextMuted = t.TextMuted
	ColorMenuText = t.MenuText
	ColorStatusText = t.StatusText
	ColorGutterText = t.GutterText

	ColorKeyword = t.Keyword
	ColorString = t.String
	ColorComment = t.Comment
	ColorNumber = t.Number
	ColorType = t.Type
	ColorFunc = t.Func

	ColorTabActive = t.TabActive
	ColorTabInactive = t.TabInactive
	ColorTabActiveBg = t.TabActiveBg
	ColorTabBarBg = t.TabBarBg
	ColorSelected = t.Selected
	ColorSelectedText = t.SelectedText

	ColorTreeText = t.TreeText
	ColorTreeSelected = t.TreeSelected

	ColorMenuDropBg = t.MenuDropBg
	ColorMenuDropText = t.MenuDropText
	ColorMenuHighlight = t.MenuHighlight
	ColorMenuHlText = t.MenuHlText
	ColorMenuShortcut = t.MenuShortcut

	ColorAccel = t.Accel
	ColorBorder = t.Border
	ColorPanelFocused = t.PanelFocused
	ColorPanelBlurred = t.PanelBlurred

	// Set markdown colors based on theme brightness
	applyMarkdownColors(t)
}

// applyMarkdownColors sets markdown preview colors appropriate for the theme.
func applyMarkdownColors(t *Theme) {
	r, g, b := t.Bg.RGB()
	brightness := (int(r)*299 + int(g)*587 + int(b)*114) / 1000

	isLight := brightness > 128

	if isLight {
		// Light theme
		ColorMarkdownH1 = tcell.NewRGBColor(180, 0, 0)     // Dark red
		ColorMarkdownH2 = tcell.NewRGBColor(0, 100, 180)    // Dark blue
		ColorMarkdownH3 = tcell.NewRGBColor(0, 130, 0)      // Dark green
		ColorMarkdownH4 = tcell.NewRGBColor(160, 80, 0)     // Dark orange
		ColorMarkdownH5 = tcell.NewRGBColor(100, 60, 150)   // Purple
		ColorMarkdownH6 = tcell.NewRGBColor(120, 120, 120)  // Gray
		ColorMarkdownLink = tcell.NewRGBColor(0, 80, 180)   // Blue
		ColorMarkdownCode = tcell.NewRGBColor(180, 60, 0)   // Dark orange
		ColorMarkdownCodeBg = tcell.NewRGBColor(230, 230, 240) // Light gray-blue
	} else {
		// Dark theme
		ColorMarkdownH1 = tcell.NewRGBColor(255, 200, 85)   // Warm yellow
		ColorMarkdownH2 = tcell.NewRGBColor(85, 200, 255)   // Cyan
		ColorMarkdownH3 = tcell.NewRGBColor(85, 220, 85)    // Green
		ColorMarkdownH4 = tcell.NewRGBColor(255, 170, 85)   // Orange
		ColorMarkdownH5 = tcell.NewRGBColor(170, 170, 255)  // Light blue
		ColorMarkdownH6 = tcell.NewRGBColor(170, 170, 170)  // Gray
		ColorMarkdownLink = tcell.NewRGBColor(80, 200, 255) // Bright cyan
		ColorMarkdownCode = tcell.NewRGBColor(255, 170, 85) // Orange
		ColorMarkdownCodeBg = tcell.NewRGBColor(30, 30, 50) // Dark blue-gray
	}
}
