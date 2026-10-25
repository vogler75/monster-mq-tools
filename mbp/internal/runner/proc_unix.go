//go:build !windows

package runner

import (
	"context"
	"os/exec"
	"syscall"
)

func buildExecCmd(ctx context.Context, command string, args []string, dir string) *exec.Cmd {
	c, a := resolvePythonScript(command, args, dir)
	cmd := exec.CommandContext(ctx, c, a...)
	cmd.Dir = dir
	prepareCmd(cmd)
	return cmd
}

func prepareCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil && cmd.Process.Pid > 0 {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
