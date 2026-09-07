//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package proc

import "os/exec"

// Configure is a no-op where process groups are not available. A cancelled
// command still stops; only its children may outlive it.
func Configure(cmd *exec.Cmd) {}
