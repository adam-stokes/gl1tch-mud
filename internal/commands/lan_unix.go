package commands

import (
	"os/exec"
	"syscall"
)

// sysProcDetach returns SysProcAttr that detaches the child from the parent's
// process group so it survives when the parent exits.
func sysProcDetach() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// ensure exec is used (imported for LookPath in lan.go)
var _ = exec.LookPath
