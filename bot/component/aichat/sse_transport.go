package aichat

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
)

// sseCommentFilterReader 过滤 SSE 响应流中的注释行与无效空行。
//
// 背景：部分网关或中转服务（如 OpenRouter 在等待上游模型就绪时）会发送形如
// ": OPENROUTER PROCESSING\n\n" 或 ": keep-alive\n\n" 的 SSE 注释帧。
// openai-go SDK 内置的 ssestream.Decoder 在解析空行 "\n\n" 时，因没有
// data 字段而产生 Data 为空的 Event，随后传给 json.Unmarshal 导致
// "unexpected end of JSON input" 错误。
//
// 本 Reader 在 HTTP 传输层剔除所有以 ':' 开头的注释行，并丢弃未跟随在有效
// "data:" 行之后的孤立空行，确保传递给 SDK 的只有完整的 SSE 事件。
type sseCommentFilterReader struct {
	reader  *bufio.Reader
	buf     bytes.Buffer
	hasData bool
}

func newSSECommentFilterReader(r io.Reader) *sseCommentFilterReader {
	return &sseCommentFilterReader{reader: bufio.NewReader(r)}
}

func (f *sseCommentFilterReader) Read(p []byte) (int, error) {
	for f.buf.Len() == 0 {
		line, err := f.reader.ReadSlice('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) == 0 {
				// 空行：仅在之前存在有效 data: 字段（构成合法 SSE 事件块）时向下传递，
				// 否则（如注释块后的空行、纯心跳空行）直接丢弃，防止 SDK 产生空事件报错
				if f.hasData {
					f.buf.Write(line)
					f.hasData = false
				}
			} else if trimmed[0] == ':' {
				// 注释行直接丢弃
			} else {
				if bytes.HasPrefix(trimmed, []byte("data:")) {
					f.hasData = true
				}
				f.buf.Write(line)
			}
		}
		if err != nil {
			if f.buf.Len() > 0 {
				break
			}
			return 0, err
		}
	}
	return f.buf.Read(p)
}

type sseFilterBody struct {
	io.Reader
	closer io.Closer
}

func (b *sseFilterBody) Close() error {
	return b.closer.Close()
}

// sseTransport 拦截 text/event-stream 响应并包装 Body 进行 SSE 过滤
type sseTransport struct {
	base http.RoundTripper
}

func (t *sseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	ct := resp.Header.Get("Content-Type")
	if bytes.Contains([]byte(ct), []byte("text/event-stream")) && resp.Body != nil {
		resp.Body = &sseFilterBody{
			Reader: newSSECommentFilterReader(resp.Body),
			closer: resp.Body,
		}
	}
	return resp, nil
}

// newOpenAIHTTPClient 返回内置 SSE 兼容过滤器的 http.Client，使用 http.DefaultTransport
// 作为底层传输，完整保留环境变量代理（HTTP_PROXY/HTTPS_PROXY）与 HTTP/2 支持。
func newOpenAIHTTPClient() *http.Client {
	return &http.Client{
		Transport: &sseTransport{base: http.DefaultTransport},
	}
}

