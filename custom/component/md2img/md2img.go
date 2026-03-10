// md2img api服务，需要先启动容器
// docker run -d -p 3000:3000 --name md2img-api jeanhua/md2img-api:latest
// 开源仓库地址: https://github.com/jeanhua/md2img-api
package md2img

import (
	"fmt"

	"github.com/go-resty/resty/v2"
)

type Md2Img struct {
	apipoint string
	client   *resty.Client
}

func NewMd2Img(apipoint string, client *resty.Client) *Md2Img {
	return &Md2Img{
		apipoint: apipoint,
		client:   client,
	}
}

func (m *Md2Img) GetImage(md string) ([]byte, error) {
	req := &Md2ImgRequest{
		Markdown: md,
	}
	resp, err := m.client.R().
		SetBody(req).
		Post(m.apipoint + "/render")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}
	return resp.Body(), nil
}

type Md2ImgRequest struct {
	Markdown string `json:"markdown"`
}
