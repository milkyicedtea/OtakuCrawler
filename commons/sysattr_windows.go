//go:build windows

package commons

import (
	"os/exec"
	"syscall"
)

func setWindowsCmdAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
