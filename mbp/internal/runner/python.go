package runner

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// resolvePythonScript runs .py commands (e.g. a component's "./make.py") through a Python 3
// interpreter, so the same command works on Windows, macOS and Linux. Other commands are
// returned unchanged.
func resolvePythonScript(command string, args []string, dir string) (string, []string) {
	if !strings.HasSuffix(strings.ToLower(command), ".py") {
		return command, args
	}
	interp, interpArgs := findPython()
	out := append(interpArgs, filepath.FromSlash(command))
	return interp, append(out, args...)
}

func findPython() (string, []string) {
	if runtime.GOOS == "windows" {
		// The py launcher is the reliable choice; "python3" is often the Microsoft Store stub.
		if p, err := exec.LookPath("py"); err == nil {
			return p, []string{"-3"}
		}
		if p, err := exec.LookPath("python"); err == nil {
			return p, nil
		}
	}
	if p, err := exec.LookPath("python3"); err == nil {
		return p, nil
	}
	return "python", nil
}
