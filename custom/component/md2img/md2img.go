package md2img

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
)

//go:embed convert.sh
var convertSh []byte

//go:embed style.css
var styleCss []byte

func init() {
	tmpDir := "./tmp"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return
	}

	convertPath := filepath.Join(tmpDir, "convert.sh")
	if _, err := os.Stat(convertPath); os.IsNotExist(err) {
		os.WriteFile(convertPath, convertSh, 0755)
	}

	stylePath := filepath.Join(tmpDir, "style.css")
	if _, err := os.Stat(stylePath); os.IsNotExist(err) {
		os.WriteFile(stylePath, styleCss, 0644)
	}
}

func GetImage(md string) ([]byte, error) {
	tmpDir := "./tmp"

	sourcePath := filepath.Join(tmpDir, "source.md")
	if err := os.WriteFile(sourcePath, []byte(md), 0644); err != nil {
		return nil, err
	}

	cmd := exec.Command("bash", "convert.sh")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	imgPath := filepath.Join(tmpDir, "output.jpg")
	data, err := os.ReadFile(imgPath)
	if err != nil {
		return nil, err
	}

	os.Remove(imgPath)

	return data, nil
}
