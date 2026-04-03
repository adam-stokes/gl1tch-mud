//go:build windows

package commands

import "syscall"

// sysProcDetach is a no-op on Windows — Setsid is not supported.
func sysProcDetach() *syscall.SysProcAttr {
	return nil
}
