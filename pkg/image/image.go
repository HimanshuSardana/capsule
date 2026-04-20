package image

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

type Image struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Images []Image

func LoadImages(path string) (Images, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading images.json: %w", err)
	}

	var images Images
	if err := json.Unmarshal(data, &images); err != nil {
		return nil, fmt.Errorf("parsing images.json: %w", err)
	}

	return images, nil
}

func GetImage(name string, imagesPath string) (Image, error) {
	images, err := LoadImages(imagesPath)
	if err != nil {
		return Image{}, err
	}

	for _, img := range images {
		if img.Name == name {
			return img, nil
		}
	}
	return Image{}, fmt.Errorf("image not found: %s", name)
}

func Download(url, path string) error {
	if IsFileURL(url) {
		return copyFile(url, path)
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func IsFileURL(url string) bool {
	return filepath.IsAbs(url) || string(url[0]) == "/" || string(url[0]) == "."
}

func copyFile(src, dst string) error {
	src = filepath.Clean(src)
	if !filepath.IsAbs(src) {
		abs, _ := filepath.Abs(src)
		src = abs
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	return err
}

func Extract(tarPath, dest string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := exec.Command("tar", "-xzf", tarPath, "-C", dest)
	cmd.Stdin = file
	return cmd.Run()
}
