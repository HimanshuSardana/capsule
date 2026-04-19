package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

type Image struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Images []Image

func main() {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		runChild(os.Args[2])
		return
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: capsule <command>")
		fmt.Println("Commands:")
		fmt.Println("  list              - List available images")
		fmt.Println("  run <image> <dir> - Run image in directory")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		listImages()
	case "run":
		if len(os.Args) < 4 {
			fmt.Println("Usage: capsule run <image> <dir>")
			os.Exit(1)
		}
		runImage(os.Args[2], os.Args[3])
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func listImages() {
	data, err := os.ReadFile("images.json")
	if err != nil {
		fmt.Printf("Error reading images.json: %v\n", err)
		os.Exit(1)
	}

	var images Images
	if err := json.Unmarshal(data, &images); err != nil {
		fmt.Printf("Error parsing images.json: %v\n", err)
		os.Exit(1)
	}

	for _, img := range images {
		fmt.Printf("%s - %s\n", img.Name, img.URL)
	}
}

func runImage(imageName, dir string) {
	img := getImage(imageName)
	if img.Name == "" {
		fmt.Printf("Image not found: %s\n", imageName)
		os.Exit(1)
	}

	fmt.Printf("Running %s in %s...\n", imageName, dir)

	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		abs, _ := filepath.Abs(dir)
		dir = abs
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		os.Exit(1)
	}

	imgPath := filepath.Join("images", imageName+".tar.gz")
	if _, err := os.Stat(imgPath); err != nil {
		fmt.Printf("Downloading image...\n")
		if err := downloadImage(img.URL, imgPath); err != nil {
			fmt.Printf("Error downloading: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Extracting image...\n")
	if err := extractTarGz(imgPath, dir); err != nil {
		fmt.Printf("Error extracting: %v\n", err)
		os.Exit(1)
	}

	runContainer(dir)
}

func getImage(name string) Image {
	data, err := os.ReadFile("images.json")
	if err != nil {
		return Image{}
	}

	var images Images
	if err := json.Unmarshal(data, &images); err != nil {
		return Image{}
	}

	for _, img := range images {
		if img.Name == name {
			return img
		}
	}
	return Image{}
}

func downloadImage(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	os.MkdirAll(filepath.Dir(path), 0755)
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractTarGz(tarPath, dest string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := exec.Command("tar", "-xzf", tarPath, "-C", dest)
	cmd.Stdin = file
	return cmd.Run()
}

func runContainer(rootfs string) {
	fmt.Println("Parent: Starting container...")

	cmd := exec.Command("/proc/self/exe", "child", rootfs)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS,
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error starting child process: %v\n", err)
		os.Exit(1)
	}
}

func runChild(rootfs string) {
	fmt.Println("Child: Inside container (PID namespace)")

	if err := syscall.Chroot(rootfs); err != nil {
		fmt.Printf("Chroot failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chdir("/"); err != nil {
		fmt.Printf("Chdir / failed: %v\n", err)
		os.Exit(1)
	}

	setupMounts()

	os.Setenv("PATH", "/bin:/usr/bin:/sbin:/usr/sbin")

	cmd := exec.Command("/bin/busybox", "sh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running shell: %v\n", err)
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