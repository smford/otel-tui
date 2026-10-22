package tui

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// CopyToClipboard attempts to copy text to the system clipboard using OSC 52
// and falls back to platform-specific utilities (pbcopy, wl-copy, xclip).
func CopyToClipboard(text string) error {
	if text == "" {
		return nil
	}

	// 1. Emit OSC 52 escape sequence (works across SSH, tmux, and modern terminal emulators)
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	osc52Seq := fmt.Sprintf("\x1b]52;c;%s\x07", encoded)
	_, _ = os.Stdout.WriteString(osc52Seq)

	// 2. Platform-specific fallback
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		}
	}

	if cmd != nil {
		in, err := cmd.StdinPipe()
		if err == nil {
			if err := cmd.Start(); err == nil {
				_, _ = in.Write([]byte(text))
				_ = in.Close()
				_ = cmd.Wait()
			}
		}
	}

	return nil
}
