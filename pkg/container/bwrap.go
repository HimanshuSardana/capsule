package container

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

var keptEnv = []string{
	"PATH",
	"HOME",
	"USER",
	"TERM",
	"TMPDIR",
	"LANG",
	"LC_ALL",
	"SHELL",
}

func clearEnviron() {
	env := os.Environ()
	keep := make(map[string]bool)
	for _, k := range keptEnv {
		keep[k] = true
	}

	for _, kv := range env {
		i := bytes.IndexByte([]byte(kv), '=')
		if i < 0 {
			continue
		}
		k := kv[:i]
		if !keep[k] {
			os.Unsetenv(k)
		}
	}
}

func ExecuteInSandbox(rootfs, codeFile, stdinFile string) (int, string, string) {
	bwrapPath, err := exec.LookPath("bwrap")
	if err != nil {
		return -1, "", fmt.Sprintf("bwrap not found: %v", err)
	}

	hostStdin := ""
	if stdinFile != "" {
		hostStdin = stdinFile
	}

	args := []string{
		"--unshare-all",
		"--setenv", "PATH", "/bin:/usr/bin:/sbin:/usr/sbin",
		"--setenv", "HOME", "/tmp",
		"--setenv", "USER", "nobody",
		"--bind", rootfs, "/",
		"--chdir", "/",
		"--proc", "/proc",
		"--dev", "/dev",
	}

	clearEnviron()

	var stdin *os.File
	if hostStdin != "" {
		f, err := os.Open(hostStdin)
		if err != nil {
			return -1, "", fmt.Sprintf("open stdin: %v", err)
		}
		stdin = f
		defer f.Close()
	}

	cmd := exec.Command(bwrapPath, args...)
	cmd.Args = append(cmd.Args, "--", "python3", codeFile)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != nil {
		cmd.Stdin = stdin
	}

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return -1, stdout.String(), fmt.Sprintf("bwrap: %v", err)
		}
	}

	return exitCode, stdout.String(), stderr.String()
}
