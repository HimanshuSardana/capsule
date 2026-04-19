package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	_ "time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		runChild()
		return
	}

	runParent()
}

func runParent() {
	fmt.Println("Parent: Starting container...")

	// Set the necessary flags for creating namespaces
	cmd := exec.Command("/proc/self/exe", "child")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS, // New PID, UTS, and Mount namespace
	}

	// Inherit the standard input/output/error to interact with the child process
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error starting child process: %v\n", err)
		os.Exit(1)
	}
}

func runChild() {
	fmt.Println("Child: Inside container (PID namespace)")

	// Chroot into the Alpine rootfs
	if err := syscall.Chroot("./testenv"); err != nil {
		fmt.Printf("Chroot failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chdir("/"); err != nil {
		fmt.Printf("Chdir / failed: %v\n", err)
		os.Exit(1)
	}

	// Mount necessary filesystems
	setupMounts()

	// Explicitly setting the PATH environment variable
	os.Setenv("PATH", "/bin:/usr/bin:/sbin:/usr/sbin")

	// Try using busybox directly (symlinks break after chroot due to absolute paths)
	cmd := exec.Command("/bin/busybox", "sh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running shell: %v\n", err)
		fmt.Println("Contents of /bin:")
		files, _ := os.ReadDir("/bin")
		for _, f := range files {
			fmt.Println("  -", f.Name())
		}
	}
}

func setupMounts() {
	syscall.Mount("", "", "", uintptr(syscall.MS_PRIVATE|syscall.MS_REC), "")

	_ = os.MkdirAll("/proc", 0o755)
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		fmt.Printf("Failed to mount /proc: %v\n", err)
	}

	_ = os.MkdirAll("/dev", 0o755)
	if err := syscall.Mount("devtmpfs", "/dev", "devtmpfs", 0, ""); err != nil {
		fmt.Printf("Failed to mount /dev: %v\n", err)
	}

	_ = os.MkdirAll("/sys", 0o755)
	if err := syscall.Mount("sysfs", "/sys", "sysfs", 0, ""); err != nil {
		fmt.Printf("Failed to mount /sys: %v\n", err)
	}

	_ = os.MkdirAll("/dev/pts", 0o755)
	if err := syscall.Mount("devpts", "/dev/pts", "devpts", 0, ""); err != nil {
		fmt.Printf("Failed to mount /dev/pts: %v\n", err)
	}
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
