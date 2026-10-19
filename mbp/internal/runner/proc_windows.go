//go:build windows

package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func buildExecCmd(ctx context.Context, command string, args []string, dir string) *exec.Cmd {
	c, a := resolveWindowsCmd(command, args, dir)
	cmd := exec.CommandContext(ctx, c, a...)
	cmd.Dir = dir
	prepareCmd(cmd)
	return cmd
}

func prepareCmd(cmd *exec.Cmd) {
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil && cmd.Process.Pid > 0 {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		_ = cmd.Process.Kill()
	}
}

func resolveWindowsCmd(command string, args []string, dir string) (string, []string) {
	c := command
	a := args
	lowerCmd := strings.ToLower(c)

	// If command ends with .sh, check if a .ps1 equivalent exists in dir
	if strings.HasSuffix(lowerCmd, ".sh") {
		base := strings.TrimSuffix(c, ".sh")
		candidate := base + ".ps1"
		cleanCandidate := filepath.Clean(candidate)
		targetPath := cleanCandidate
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(dir, cleanCandidate)
		}
		if _, err := os.Stat(targetPath); err == nil {
			c = candidate
			lowerCmd = strings.ToLower(c)
		} else if bashPath := findBash(); bashPath != "" {
			bashArgs := []string{c}
			bashArgs = append(bashArgs, a...)
			return bashPath, bashArgs
		}
	}

	if strings.HasSuffix(lowerCmd, ".ps1") {
		psExe := "powershell.exe"
		if p, err := exec.LookPath("pwsh.exe"); err == nil {
			psExe = p
		} else if p, err := exec.LookPath("powershell.exe"); err == nil {
			psExe = p
		}
		scriptFile := filepath.FromSlash(c)
		psArgs := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptFile}
		psArgs = append(psArgs, a...)
		return psExe, psArgs
	}

	return c, a
}

func findBash() string {
	if p, err := exec.LookPath("bash.exe"); err == nil {
		return p
	}
	if p, err := exec.LookPath("bash"); err == nil {
		return p
	}
	gitBash := filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe")
	if _, err := os.Stat(gitBash); err == nil {
		return gitBash
	}
	return ""
}
