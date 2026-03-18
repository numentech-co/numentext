package graphics

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

// maxSixelColors is the maximum number of colors in a Sixel palette.
const maxSixelColors = 256

// EncodeSixel converts an RGBA image to a Sixel escape sequence string.
// The image is quantized to at most 256 colors (Sixel limit).
// Pixel rows are encoded in groups of 6 (the Sixel row format).
//
// The returned string includes the DCS introducer and ST terminator:
//   \ePq ... data ... \e\\
func EncodeSixel(img *image.RGBA) string {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w == 0 || h == 0 {
		return ""
	}

	// Build a color palette by collecting unique colors, capping at maxSixelColors.
	palette, colorIndex := buildPalette(img)

	var sb strings.Builder

	// DCS introducer: ESC P q
	sb.WriteString("\x1bPq")

	// Emit palette definitions: #N;2;R;G;B (percentages 0-100)
	for i, c := range palette {
		r, g, b, _ := c.RGBA()
		rp := int(r>>8) * 100 / 255
		gp := int(g>>8) * 100 / 255
		bp := int(b>>8) * 100 / 255
		fmt.Fprintf(&sb, "#%d;2;%d;%d;%d", i, rp, gp, bp)
	}

	// Encode pixels in bands of 6 rows.
	for band := 0; band*6 < h; band++ {
		y0 := band * 6

		// For each color used in this band, emit a row of sixel data.
		usedColors := bandColors(img, y0, w, h, colorIndex)
		for ci, colorIdx := range usedColors {
			// Select color register.
			fmt.Fprintf(&sb, "#%d", colorIdx)

			// Build the sixel characters for this color across the width.
			for x := 0; x < w; x++ {
				sixVal := byte(0)
				for bit := 0; bit < 6; bit++ {
					y := y0 + bit
					if y >= h {
						break
					}
					px := img.RGBAAt(bounds.Min.X+x, bounds.Min.Y+y)
					if px.A < 128 {
						continue // treat transparent as background
					}
					idx := colorIndex[colorKey(px)]
					if idx == colorIdx {
						sixVal |= 1 << uint(bit)
					}
				}
				sb.WriteByte(sixVal + 63) // Sixel char = value + 63
			}

			// After last color in band, emit '$' (carriage return) or '-' (new line).
			if ci < len(usedColors)-1 {
				sb.WriteByte('$') // CR: return to start of this sixel row
			}
		}

		// Graphics new line (move down 6 pixels).
		if (band+1)*6 < h {
			sb.WriteByte('-')
		}
	}

	// ST terminator: ESC backslash
	sb.WriteString("\x1b\\")

	return sb.String()
}

// colorKey produces a uint32 key from an RGBA color for map lookups.
func colorKey(c color.RGBA) uint32 {
	return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
}

// buildPalette extracts up to maxSixelColors unique opaque colors from img.
// If there are more than maxSixelColors colors, it uses a simple
// nearest-color quantization by reducing to 6-bit color channels.
func buildPalette(img *image.RGBA) ([]color.RGBA, map[uint32]int) {
	bounds := img.Bounds()
	seen := make(map[uint32]color.RGBA)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			px := img.RGBAAt(x, y)
			if px.A < 128 {
				continue
			}
			key := colorKey(px)
			if _, ok := seen[key]; !ok {
				seen[key] = px
			}
		}
	}

	// If within limit, use exact colors.
	if len(seen) <= maxSixelColors {
		palette := make([]color.RGBA, 0, len(seen))
		index := make(map[uint32]int, len(seen))
		for key, c := range seen {
			index[key] = len(palette)
			palette = append(palette, c)
		}
		return palette, index
	}

	// Quantize by reducing to 6-bit color (64 levels per channel -> max 262144,
	// but in practice far fewer). Then map original colors to nearest quantized.
	quantized := make(map[uint32]color.RGBA)
	for _, c := range seen {
		qc := color.RGBA{
			R: (c.R >> 2) << 2,
			G: (c.G >> 2) << 2,
			B: (c.B >> 2) << 2,
			A: 255,
		}
		qk := colorKey(qc)
		quantized[qk] = qc
	}

	// If still over limit, reduce further to 4-bit.
	if len(quantized) > maxSixelColors {
		quantized = make(map[uint32]color.RGBA)
		for _, c := range seen {
			qc := color.RGBA{
				R: (c.R >> 4) << 4,
				G: (c.G >> 4) << 4,
				B: (c.B >> 4) << 4,
				A: 255,
			}
			qk := colorKey(qc)
			quantized[qk] = qc
		}
	}

	palette := make([]color.RGBA, 0, len(quantized))
	qIndex := make(map[uint32]int, len(quantized))
	for key, c := range quantized {
		qIndex[key] = len(palette)
		palette = append(palette, c)
	}

	// Build original->palette index mapping.
	index := make(map[uint32]int, len(seen))
	for key, c := range seen {
		qc := color.RGBA{
			R: (c.R >> 2) << 2,
			G: (c.G >> 2) << 2,
			B: (c.B >> 2) << 2,
			A: 255,
		}
		qk := colorKey(qc)
		if _, ok := qIndex[qk]; !ok {
			// Was further quantized to 4-bit.
			qc = color.RGBA{
				R: (c.R >> 4) << 4,
				G: (c.G >> 4) << 4,
				B: (c.B >> 4) << 4,
				A: 255,
			}
			qk = colorKey(qc)
		}
		index[key] = qIndex[qk]
	}

	return palette, index
}

// bandColors returns the distinct color indices used in the 6-pixel-tall band
// starting at y0.
func bandColors(img *image.RGBA, y0, w, h int, colorIndex map[uint32]int) []int {
	bounds := img.Bounds()
	seen := make(map[int]bool)
	var result []int

	for bit := 0; bit < 6; bit++ {
		y := y0 + bit
		if y >= h {
			break
		}
		for x := 0; x < w; x++ {
			px := img.RGBAAt(bounds.Min.X+x, bounds.Min.Y+y)
			if px.A < 128 {
				continue
			}
			idx := colorIndex[colorKey(px)]
			if !seen[idx] {
				seen[idx] = true
				result = append(result, idx)
			}
		}
	}
	return result
}
