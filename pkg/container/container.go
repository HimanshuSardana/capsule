package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
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
	defer cleanupMounts()

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
	defer cleanupMounts()

	os.Setenv("PATH", "/bin:/usr/bin:/sbin:/usr/sbin")

	cmd := exec.Command("python3", codeFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if inContainerStdin != "" {
		f, err := os.Open(inContainerStdin)
		if err == nil {
			cmd.Stdin = f
			defer f.Close()
		}
	}

	if err := applySeccomp(); err != nil {
		return fmt.Errorf("seccomp: %w", err)
	}

	return cmd.Run()
}

const (
	PR_SET_SECCOMP       = 22
	SECCOMP_MODE_FILTER  = 2
	SECCOMP_RET_ALLOW    = 0x7ffc0000
	SECCOMP_RET_KILL     = 0x00000000
)

var allowedSyscalls = []uint32{
	0,   // read
	1,   // write
	3,   // close
	8,   // lseek
	9,   // mmap
	12,  // brk
	15,  // rt_sigreturn
	60,  // exit
	61,  // getdents64
	158, // arch_prctl
	231, // execve
	257, // openat
	262, // newfstatat
	273, // futex
}

func applySeccomp() error {
	n := len(allowedSyscalls)
	instrs := make([]unix.SockFilter, 2+n+2)

	instrs[0] = unix.SockFilter{
		Code: unix.BPF_LD | unix.BPF_W | unix.BPF_ABS,
		K:    0,
	}

	for i := 0; i < n; i++ {
		jt := uint8(0)
		jf := uint8(n - i - 1 + 2)
		if i == n-1 {
			jf = 0
		}
		instrs[1+i] = unix.SockFilter{
			Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_JGT,
			Jt:   jt,
			Jf:   jf,
			K:    allowedSyscalls[i],
		}
	}

	instrs[2+n] = unix.SockFilter{
		Code: unix.BPF_RET | unix.BPF_K,
		K:    SECCOMP_RET_KILL,
	}
	instrs[2+n+1] = unix.SockFilter{
		Code: unix.BPF_RET | unix.BPF_K,
		K:    SECCOMP_RET_ALLOW,
	}

	prog := &unix.SockFprog{
		Len:    uint16(len(instrs)),
		Filter: (*unix.SockFilter)(unsafe.Pointer(&instrs[0])),
	}

	if err := unix.Prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, uintptr(unsafe.Pointer(prog)), 0, 0); err != nil {
		return fmt.Errorf("prctl: %w", err)
	}
	return nil
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

func cleanupMounts() {
	syscall.Unmount("/proc", 0)
	syscall.Unmount("/dev/pts", 0)
}

func writeResolvConf() error {
	cmd := exec.Command("sh", "-c", "echo 'nameserver 8.8.8.8' > /etc/resolv.conf")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
