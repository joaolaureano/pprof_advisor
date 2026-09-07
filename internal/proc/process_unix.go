//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package proc

import (
	"os/exec"
	"syscall"
)

// Configure puts the command in its own process group and kills the whole
// group on cancellation, so a timeout reaps the test binary or the compiler
// the go tool spawned rather than orphaning it.
func Configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
