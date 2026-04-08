package graphics

import (
	"os"
	"golang.org/x/sys/unix"
)

// CellSize returns the pixel dimensions of a single terminal cell (width, height).
// Uses TIOCGWINSZ ioctl to get both character and pixel dimensions of the terminal.
// Falls back to (8, 16) if the query fails or returns zero.
func CellSize() (cellW, cellH int) {
	cellW, cellH = 8, 16 // defaults

	f, err := os.Open("/dev/tty")
	if err != nil {
		return
	}
	defer f.Close()

	ws, err := unix.IoctlGetWinsize(int(f.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return
	}

	if ws.Col > 0 && ws.Xpixel > 0 {
		cellW = int(ws.Xpixel) / int(ws.Col)
	}
	if ws.Row > 0 && ws.Ypixel > 0 {
		cellH = int(ws.Ypixel) / int(ws.Row)
	}

	if cellW < 1 {
		cellW = 8
	}
	if cellH < 1 {
		cellH = 16
	}

	return
}
