package services

import (
	"encoding/binary"
	"time"
)

// pngDims reads width/height from a PNG header; returns (0,0) when not a PNG.
func pngDims(png []byte) (int, int) {
	sig := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if len(png) < 24 || string(png[:8]) != string(sig) || string(png[12:16]) != "IHDR" {
		return 0, 0
	}
	return int(binary.BigEndian.Uint32(png[16:20])), int(binary.BigEndian.Uint32(png[20:24]))
}

// timestamp returns a compact local timestamp for default file names.
func timestamp() string {
	return time.Now().Format("20060102-150405")
}
