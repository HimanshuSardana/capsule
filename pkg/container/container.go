package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

type Runtime struct {
	rootfs string
}

func NewRuntime(rootfs string) *Runtime {
	return &Runtime{rootfs: rootfs}
}

func (r *Runtime) Run() error {
	timeStart := time.Now()

	cmd := exec.Command("/proc/self/exe", "child", r.rootfs)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS,
	}
	duration := time.Since(timeStart)
	fmt.Printf("Container started in %v\n", duration)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func RunChild(rootfs string) error {
	if err := syscall.Chroot(rootfs); err != nil {
		return fmt.Errorf("chroot failed: %w", err)
	}

	if err := syscall.Sethostname([]byte("capsule")); err != nil {
		return fmt.Errorf("sethostname failed: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / failed: %w", err)
	}

	if err := setupMounts(); err != nil {
		return err
	}

	os.Setenv("PATH", "/bin:/usr/bin:/sbin:/usr/sbin")

	if err := writeResolvConf(); err != nil {
		fmt.Printf("Error setting resolv.conf: %v\n", err)
	}

	cmd := exec.Command("/bin/busybox", "sh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunCode(rootfs, codeFile, stdinFile string) error {
	inContainerStdin := strings.TrimPrefix(stdinFile, rootfs)

	if err := syscall.Chroot(rootfs); err != nil {
		return fmt.Errorf("chroot failed: %w", err)
	}

	if err := syscall.Sethostname([]byte("capsule")); err != nil {
		return fmt.Errorf("sethostname failed: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / failed: %w", err)
	}

	if err := setupMounts(); err != nil {
		return err
	}

	os.Setenv("PATH", "/bin:/usr/bin:/sbin:/usr/sbin")

	fmt.Fprintf(os.Stderr, "[RUN-CODE] stdinFile=%s codeFile=%s\n", stdinFile, codeFile)
	cmd := exec.Command("python3", codeFile)

	if inContainerStdin != "" {
		f, err := os.Open(inContainerStdin)
		if err == nil {
			cmd.Stdin = f
			defer f.Close()
		}
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func setupMounts() error {
	syscall.Mount("", "", "", uintptr(syscall.MS_PRIVATE|syscall.MS_REC), "")

	if err := os.MkdirAll("/proc", 0o755); err != nil {
		return fmt.Errorf("mkdir /proc: %w", err)
	}
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("mount /proc: %w", err)
	}

	if err := os.MkdirAll("/dev/pts", 0o755); err != nil {
		return fmt.Errorf("mkdir /dev/pts: %w", err)
	}
	if err := syscall.Mount("devpts", "/dev/pts", "devpts", 0, ""); err != nil {
		return fmt.Errorf("mount /dev/pts: %w", err)
	}

	// Create /dev/null if it doesn't exist
	if err := os.MkdirAll("/dev", 0o755); err != nil {
		return fmt.Errorf("mkdir /dev: %w", err)
	}
	nullFile, err := os.OpenFile("/dev/null", os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return fmt.Errorf("create /dev/null: %w", err)
	}
	nullFile.Close()

	return nil
}

func writeResolvConf() error {
	cmd := exec.Command("sh", "-c", "echo 'nameserver 8.8.8.8' > /etc/resolv.conf")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
