package md2img

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

//go:embed style.css
var styleCss []byte

func init() {
	tmpDir := "./tmp"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return
	}

	stylePath := filepath.Join(tmpDir, "style.css")
	if _, err := os.Stat(stylePath); os.IsNotExist(err) {
		os.WriteFile(stylePath, styleCss, 0644)
	}
}

func GetImage(md string) ([]byte, error) {
	tmpDir := "./tmp"

	absTmpDir, err := filepath.Abs(tmpDir)
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()

	sourcePath := filepath.Join(absTmpDir, id+".md")
	if err := os.WriteFile(sourcePath, []byte(md), 0644); err != nil {
		return nil, err
	}
	defer os.Remove(sourcePath)

	imgPath := filepath.Join(absTmpDir, id+".jpg")
	htmlPath := filepath.Join(absTmpDir, id+".html")
	defer os.Remove(htmlPath)

	stylePath := filepath.Join(absTmpDir, "style.css")

	cmd := exec.Command("pandoc", sourcePath, "-o", htmlPath, "--css", stylePath, "--standalone", "--embed-resources")
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	cmd = exec.Command("wkhtmltoimage", "--width", "750", "--quality", "100", htmlPath, imgPath)
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(imgPath)
	if err != nil {
		return nil, err
	}

	os.Remove(imgPath)

	return data, nil
}
