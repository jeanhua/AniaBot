// 使用 puppeteer 转换 markdown 为图片，使用前需要安装如下依赖：
// npm install puppeteer-core highlight.js marked
// 需安装 google-chrome
/*
	wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
	sudo apt install ./google-chrome-stable_current_amd64.deb -y
*/
// 提前启动 chrome 浏览器
/*
	google-chrome --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-profile-snap --headless=new --no-sandbox --disable-gpu --disable-dev-shm-usage
*/
package md2img

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

//go:embed snap.js
var snapScript []byte

func init() {
	tmpDir := "./tmp"
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return
	}

	snapPath := filepath.Join(tmpDir, "snap.js")
	os.WriteFile(snapPath, snapScript, 0644)
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

	imgPath := filepath.Join(absTmpDir, id+".png")
	snapPath := filepath.Join(absTmpDir, "snap.js")

	cmd := exec.Command("node", snapPath, sourcePath, imgPath)
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
