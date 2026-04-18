package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func main() {
	rootfs := "/tmp/rootfs"

	os.RemoveAll(rootfs)
	if err := os.MkdirAll(rootfs, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create rootfs: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(rootfs+"/usr/sbin", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create /usr/sbin: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(rootfs+"/usr/lib", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create /usr/lib: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(rootfs+"/usr/lib64", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create /lib64: %v\n", err)
		os.Exit(1)
	}

	bashPath := "/usr/sbin/bash"
	if err := copyFile(bashPath, rootfs+bashPath); err != nil {
		fmt.Fprintf(os.Stderr, "failed to copy bash: %v\n", err)
		os.Exit(1)
	}

	libs := map[string]string{
		"/usr/lib/libreadline.so.8":       "/usr/lib/libreadline.so.8",
		"/usr/lib/libc.so.6":              "/usr/lib/libc.so.6",
		"/usr/lib/libncursesw.so.6":       "/usr/lib/libncursesw.so.6",
		"/usr/lib64/ld-linux-x86-64.so.2": "/lib64/ld-linux-x86-64.so.2",
	}

	for src, dstPath := range libs {
		dest := filepath.Join(rootfs, dstPath)

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "mkdir failed: %v\n", err)
			os.Exit(1)
		}

		if err := copyFile(src, dest); err != nil {
			fmt.Fprintf(os.Stderr, "failed to copy %s: %v\n", src, err)
			os.Exit(1)
		}
	}

	if err := os.MkdirAll(rootfs+"/lib", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create lib: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(rootfs+"/lib64", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create lib64: %v\n", err)
		os.Exit(1)
	}

	if err := syscall.Chroot(rootfs); err != nil {
		fmt.Fprintf(os.Stderr, "chroot failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr, "chdir failed: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("/usr/sbin/bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "command failed: %v\n", err)
		os.Exit(1)
	}
}

func copyFile(src, dst string) error {
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

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

