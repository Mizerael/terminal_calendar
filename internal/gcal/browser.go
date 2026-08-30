package gcal

import (
	"os/exec"
	"runtime"
)

// openBrowser tries to open a URL in the user's default browser on macOS,
// Windows and Linux. It is best-effort: failures are ignored and the URL
// stays printed for manual copy-paste.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		var err error
		cmd, err = firstAvailable("xdg-open", "sensible-browser", "x-www-browser", "firefox", "chromium", "google-chrome")
		if err != nil {
			return err
		}
		cmd.Args = append(cmd.Args, url)
	}
	return cmd.Start()
}

func firstAvailable(bins ...string) (*exec.Cmd, error) {
	for _, b := range bins {
		if p, err := exec.LookPath(b); err == nil {
			return exec.Command(p), nil
		}
	}
	return nil, &exec.Error{Name: bins[0], Err: exec.ErrNotFound}
}
