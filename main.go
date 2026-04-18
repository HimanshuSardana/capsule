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

var logFile *os.File

func main() {
	// log("Starting capsule")

	rootfs := "/tmp/rootfs"

	// log("Removing rootfs: %v", os.RemoveAll(rootfs))
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

	binaries := []string{"/usr/sbin/bash", "/usr/sbin/ls", "/usr/sbin/cat", "/usr/sbin/vim"}
	requiredFiles := []string{"/etc", "/etc/passwd", "/etc/group", "/etc/hosts", "/etc/hostname", "/home", "/usr/bin/python3.14"}
	copiedLibs := make(map[string]bool)

	for _, req := range requiredFiles {
		src := req
		dst := rootfs + req
		if req == "/home" {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				// log("mkdir %s: %v", dst, err)
			}
			continue
		}
		info, err := os.Stat(src)
		if err != nil {
			// log("stat %s: %v", src, err)
			continue
		}
		if info.IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				// log("mkdir %s: %v", dst, err)
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				// log("copy %s: %v", src, err)
			}
		}
	}

	for _, bin := range binaries {
		// log("Copying binary: %s -> %s", bin, rootfs+bin)
		if err := copyFile(bin, rootfs+bin); err != nil {
			// log("failed to copy %s: %v", bin, err)
			os.Exit(1)
		}

		output, err := exec.Command("ldd", bin).Output()
		if err != nil {
			// log("ldd failed for %s: %v", bin, err)
			os.Exit(1)
		}

		// log("ldd output for %s:\n%s", bin, string(output))

		libs := parseLdd(string(output))
		// log("Parsed libs for %s: %v", bin, libs)
		for _, lib := range libs {
			if !copiedLibs[lib] {
				copiedLibs[lib] = true
				// log("Copying lib: %s -> %s", lib, rootfs+lib)
				if err := copyFile(lib, rootfs+lib); err != nil {
					// log("failed to copy %s: %v", lib, err)
					os.Exit(1)
				}
			}
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

	os.RemoveAll(rootfs + "/lib")
	if err := os.Symlink("usr/lib", rootfs+"/lib"); err != nil {
		// log("symlink lib: %v", err)
	} else {
		// log("symlink created: /lib -> usr/lib")
	}

	os.RemoveAll(rootfs + "/lib64")
	if err := os.Symlink("usr/lib64", rootfs+"/lib64"); err != nil {
		// log("symlink lib64: %v", err)
	} else {
		// log("symlink created: /lib64 -> usr/lib64")
	}

	os.RemoveAll(rootfs + "/bin")
	if err := os.Symlink("usr/sbin", rootfs+"/bin"); err != nil {
		// log("symlink bin: %v", err)
	} else {
		// log("symlink created: /bin -> usr/sbin")
	}

	os.RemoveAll(rootfs + "/usr/bin")
	if err := os.Symlink("../usr/sbin", rootfs+"/usr/bin"); err != nil {
		// log("symlink usr/bin: %v", err)
	} else {
		// log("symlink created: /usr/bin -> usr/sbin")
	}

	if err := syscall.Chroot(rootfs); err != nil {
		fmt.Fprintf(os.Stderr, "chroot failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chdir("/"); err != nil {
		fmt.Fprintf(os.Stderr, "chdir failed: %v\n", err)
		os.Exit(1)
	}

	// testCmd := exec.Command("/usr/sbin/ls", "-la", ".")
	// testCmd.Stdin = os.Stdin
	// testCmd.Stdout = os.Stdout
	// testCmd.Stderr = os.Stderr

	// log("Testing ls inside chroot...")
	// if err := testCmd.Run(); err != nil {
	// 	// log("ls test failed: %v", err)
	// 	if exitErr, ok := err.(*exec.ExitError); ok {
	// 		// log("ls stderr: %s", string(exitErr.Stderr))
	// 		fmt.Fprintf(os.Stderr, "ls test failed: %v\n", exitErr)
	// 	}
	// }

	cmd := exec.Command("/usr/sbin/bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "command failed: %v\n", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "stderr: %s\n", string(exitErr.Stderr))
		}
		os.Exit(1)
	}
}

func copyFile(src, dst string) error {
	// log("copyFile: %s -> %s", src, dst)

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir failed: %v", err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dst: %v", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy: %v", err)
	}

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat: %v", err)
	}

	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("chmod: %v", err)
	}

	// log("copyFile success: %s", dst)
	return nil
}

// func log(format string, args ...interface{}) {
// 	fmt.Println(fmt.Sprintf(format, args...))
// }

func parseLdd(output string) []string {
	lines := strings.Split(output, "\n")
	libs := []string{}

	for _, line := range lines {
		parts := strings.Split(line, "=>")
		if len(parts) != 2 {
			continue
		}

		path := strings.TrimSpace(strings.Split(parts[1], "(")[0])
		if strings.HasPrefix(path, "/") {
			libs = append(libs, path)
		}
	}

	return libs
}
