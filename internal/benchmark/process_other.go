//go:build !(aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris)

package benchmark

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}
