//go:build darwin && cgo

package clipboard

/*
#cgo LDFLAGS: -framework Foundation -framework AppKit -mmacosx-version-min=12.0
#include <stdlib.h>

long wm_pb_change_count(void);
int  wm_pb_read_text(void **out);
int  wm_pb_read_image(void **out);
int  wm_pb_read_files(char **out);
void wm_pb_free(void *p);
*/
import "C"

import (
	"strings"
	"unsafe"
)

// readOSClipboard reads the macOS general pasteboard and classifies its content.
// File URLs win over an image, which wins over text, so a Finder copy of a file
// is recorded as a file rather than its icon preview. The pasteboard change
// count is returned so callers can skip the read when nothing changed.
func readOSClipboard() (Capture, int64) {
	count := int64(C.wm_pb_change_count())

	var fout *C.char
	if n := C.wm_pb_read_files(&fout); n > 0 && fout != nil {
		b := C.GoBytes(unsafe.Pointer(fout), n)
		C.wm_pb_free(unsafe.Pointer(fout))
		if c := FileCapture(strings.Split(string(b), "\n")); c.Text != "" {
			return c, count
		}
	}

	var iout unsafe.Pointer
	if n := C.wm_pb_read_image(&iout); n > 0 && iout != nil {
		b := C.GoBytes(iout, n)
		C.wm_pb_free(iout)
		if len(b) <= MaxImageBytes {
			return ImageCapture(b), count
		}
	}

	var tout unsafe.Pointer
	if n := C.wm_pb_read_text(&tout); n > 0 && tout != nil {
		b := C.GoBytes(tout, n)
		C.wm_pb_free(tout)
		return TextCapture(string(b)), count
	}

	return Capture{}, count
}
