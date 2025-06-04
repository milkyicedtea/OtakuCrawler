//go:build !windows

package commons

import "os/exec"

func setWindowsCmdAttrs(cmd *exec.Cmd) {
	// No-op on non-Windows
}
