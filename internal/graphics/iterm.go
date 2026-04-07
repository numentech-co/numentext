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
// Format: ESC ] 1337 ; File=inline=1;width=Xpx;height=Ypx;preserveAspectRatio=1 : BASE64 BEL
//
// widthCells and heightCells specify the cell dimensions to occupy.
func EncodeITerm(img image.Image, widthCells, heightCells int) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("png encode: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	// iTerm2 inline image protocol
	// width and height in cells so the image respects cell grid boundaries
	seq := fmt.Sprintf("\033]1337;File=inline=1;width=%d;height=%d;preserveAspectRatio=1:%s\a",
		widthCells, heightCells, b64)

	return seq, nil
}
