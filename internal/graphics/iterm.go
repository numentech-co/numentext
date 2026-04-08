package graphics

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"bytes"
)

// EncodeITerm encodes an image using iTerm2's inline image protocol.
// The protocol places images cleanly within terminal cells without pixel bleeding.
//
// Format: ESC ] 1337 ; File=inline=1;size=N;width=Npx;height=Npx;preserveAspectRatio=1 : BASE64 BEL
//
// imgWidth and imgHeight are the image dimensions in pixels.
// heightCells is the number of terminal rows to occupy.
func EncodeITerm(img image.Image, widthCells, heightCells int) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("png encode: %w", err)
	}

	data := buf.Bytes()
	b64 := base64.StdEncoding.EncodeToString(data)

	bounds := img.Bounds()
	pxW := bounds.Dx()
	pxH := bounds.Dy()

	// Use pixel dimensions for precise sizing.
	// size= is the byte count of the file data (helps iTerm2 allocate memory).
	seq := fmt.Sprintf("\033]1337;File=inline=1;size=%d;width=%dpx;height=%dpx;preserveAspectRatio=1:%s\a",
		len(data), pxW, pxH, b64)

	return seq, nil
}
