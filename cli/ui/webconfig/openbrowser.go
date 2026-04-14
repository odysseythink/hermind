package webconfig

import (
	"os/exec"
	"runtime"
)

// OpenBrowser launches the user's default browser at url. Best-effort; a
// failure is non-fatal because the URL is also printed to stderr.
func OpenBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
