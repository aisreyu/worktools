//go:build windows

package screenshot

import (
	"fmt"
	"os/exec"
)

// copyImageFile loads a PNG file into the Windows clipboard as a bitmap using
// PowerShell and the System.Windows.Forms / System.Drawing assemblies.
func copyImageFile(path string) error {
	ps := fmt.Sprintf(
		`Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing; `+
			`$img=[System.Drawing.Image]::FromFile('%s'); [System.Windows.Forms.Clipboard]::SetImage($img); $img.Dispose()`,
		path,
	)
	return exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps).Run()
}
