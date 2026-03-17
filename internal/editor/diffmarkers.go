package editor

import (
	"github.com/gdamore/tcell/v2"
)

// DiffChangeType represents the type of change for a line in a diff.
// These markers can be set by plugins (e.g., numentext-git) via the
// numen.set_gutter_markers host API.
type DiffChangeType int

const (
	DiffAdded    DiffChangeType = iota // new line added
	DiffModified                       // line modified
	DiffDeleted                        // line was deleted (marker at this position)
)

// diffMarkerChar returns the marker character and foreground color for a diff change type.
func diffMarkerChar(changeType DiffChangeType) (rune, tcell.Color) {
	switch changeType {
	case DiffAdded:
		return '+', tcell.ColorGreen
	case DiffModified:
		return '~', tcell.ColorBlue
	case DiffDeleted:
		return '-', tcell.ColorRed
	default:
		return ' ', tcell.ColorWhite
	}
}
