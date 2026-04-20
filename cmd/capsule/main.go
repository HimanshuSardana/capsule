package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/HimanshuSardana/capsule/pkg/container"
	"github.com/HimanshuSardana/capsule/pkg/image"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "child" {
		if err := container.RunChild(os.Args[2]); err != nil {
			fmt.Printf("Error running child: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		listImages()
	case "run":
		runImage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: capsule <command>")
	fmt.Println("Commands:")
	fmt.Println("  list              - List available images")
	fmt.Println("  run <image> <dir> - Run image in directory")
}

func listImages() {
	images, err := image.LoadImages("images.json")
	if err != nil {
		fmt.Printf("Error loading images: %v\n", err)
		os.Exit(1)
	}

	for _, img := range images {
		fmt.Printf("%s - %s\n", img.Name, img.URL)
	}
}

func runImage() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: capsule run <image> <dir>")
		os.Exit(1)
	}

	imageName := os.Args[2]
	dir := os.Args[3]

	img, err := image.GetImage(imageName, "images.json")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Running %s in %s...\n", imageName, dir)

	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		abs, _ := filepath.Abs(dir)
		dir = abs
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		os.Exit(1)
	}

	imgPath := filepath.Join("images", imageName+".tar.gz")
	if _, err := os.Stat(imgPath); err != nil {
		fmt.Printf("Downloading image...\n")
		if err := image.Download(img.URL, imgPath); err != nil {
			fmt.Printf("Error downloading: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Extracting image...\n")
	if err := image.Extract(imgPath, dir); err != nil {
		fmt.Printf("Error extracting: %v\n", err)
		os.Exit(1)
	}

	runtime := container.NewRuntime(dir)
	if err := runtime.Run(); err != nil {
		fmt.Printf("Error running container: %v\n", err)
		os.Exit(1)
	}
}