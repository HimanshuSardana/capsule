package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func main() {
	rootfs := "/home/himanshu/personal/projects/capsule/testenv"

	if err := os.MkdirAll(rootfs, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create rootfs: %v\n", err)
		os.Exit(1)
	}

	dirs := []string{"/usr/sbin", "/usr/lib", "/usr/lib64", "/proc", "/lib", "/lib64", "/bin", "/usr/bin", "/etc", "/home"}
	for _, d := range dirs {
		if err := os.MkdirAll(rootfs+d, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", d, err)
			os.Exit(1)
		}
	}

	binaries := []string{"/usr/sbin/bash", "/usr/sbin/ls", "/usr/sbin/cat", "/usr/sbin/vim", "/usr/sbin/ps"}
	requiredFiles := []string{"/etc/passwd", "/etc/group", "/etc/hosts", "/etc/hostname"}

	copiedLibs := make(map[string]bool)

	for _, req := range requiredFiles {
		if err := copyFile(req, rootfs+req); err != nil {
			fmt.Fprintf(os.Stderr, "copy %s: %v\n", req, err)
		}
	}

	for _, bin := range binaries {
		if err := copyFile(bin, rootfs+bin); err != nil {
			fmt.Fprintf(os.Stderr, "copy binary %s: %v\n", bin, err)
			os.Exit(1)
		}

		output, err := exec.Command("ldd", bin).Output()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ldd %s: %v\n", bin, err)
			continue
		}

		for _, lib := range parseLdd(string(output)) {
			if !copiedLibs[lib] {
				copiedLibs[lib] = true
				if err := copyFile(lib, rootfs+lib); err != nil {
					fmt.Fprintf(os.Stderr, "copy lib %s: %v\n", lib, err)
				}
			}
		}
	}

	symlinks := map[string]string{
		rootfs + "/lib":     "usr/lib",
		rootfs + "/lib64":   "usr/lib64",
		rootfs + "/bin":     "usr/sbin",
		rootfs + "/usr/bin": "../usr/sbin",
	}
	for dst, target := range symlinks {
		os.RemoveAll(dst)
		if err := os.Symlink(target, dst); err != nil {
			fmt.Fprintf(os.Stderr, "symlink %s -> %s: %v\n", dst, target, err)
		}
	}

	cmd := exec.Command("/usr/sbin/bash", "--noprofile", "--norc")

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "/"

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS,
	}

	setup := fmt.Sprintf(`
		mount -t proc proc /proc || echo "proc mount failed"
		chroot %s /usr/sbin/bash --noprofile --norc
	`, rootfs)

	cmd = exec.Command("/usr/sbin/bash", "-c", setup)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWUTS,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("Starting container (bash should be PID 1 inside)...")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "container failed: %v\n", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "exit status: %d\n", exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	if info, err := srcFile.Stat(); err == nil {
		os.Chmod(dst, info.Mode())
	}
	return nil
}

func parseLdd(output string) []string {
	lines := strings.Split(output, "\n")
	libs := []string{}
	for _, line := range lines {
		if idx := strings.Index(line, "=>"); idx != -1 {
			part := strings.TrimSpace(strings.Split(line[idx+2:], "(")[0])
			if strings.HasPrefix(part, "/") {
				libs = append(libs, part)
			}
		}
	}
	return libs
}
