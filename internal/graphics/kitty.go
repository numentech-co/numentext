package graphics

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"
)

// kittyChunkSize is the maximum number of base64 bytes per Kitty protocol
// chunk. The Kitty spec recommends 4096.
const kittyChunkSize = 4096

// EncodeKitty converts an RGBA image to a Kitty graphics protocol escape
// sequence string. The image is PNG-encoded, then base64-encoded and split
// into chunks sent via APC sequences.
//
// Chunk format:
//   First/middle: \e_Gf=100,t=d,s=W,v=H,m=1;BASE64_CHUNK\e\\
//   Final:        \e_Gf=100,t=d,s=W,v=H,m=0;BASE64_CHUNK\e\\
//
// f=100 means PNG format, t=d means direct data transfer.
func EncodeKitty(img *image.RGBA) (string, error) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w == 0 || h == 0 {
		return "", nil
	}

	// Encode the image as PNG.
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("png encode: %w", err)
	}

	// Base64 encode the PNG data.
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	var sb strings.Builder

	// Split into chunks.
	for i := 0; i < len(b64); i += kittyChunkSize {
		end := i + kittyChunkSize
		if end > len(b64) {
			end = len(b64)
		}
		chunk := b64[i:end]
		isLast := end >= len(b64)

		m := 1
		if isLast {
			m = 0
		}

		if i == 0 {
			// First chunk includes format and size metadata.
			fmt.Fprintf(&sb, "\x1b_Gf=100,t=d,s=%d,v=%d,m=%d;%s\x1b\\", w, h, m, chunk)
		} else {
			// Continuation chunks only specify m.
			fmt.Fprintf(&sb, "\x1b_Gm=%d;%s\x1b\\", m, chunk)
		}
	}

	return sb.String(), nil
}
