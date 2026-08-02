package qqofficial

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// md510mWindow md5_10m 校验的窗口大小：前 10002432 字节（约 10MB，官方定义）。
const md510mWindow = 10002432

// uploadMedia 上传媒体资源换取 file_info：
//   - http(s) URL → URL 直传（平台下载转存），一次调用完成；
//   - base64:// / data: / file:// → 分片上传（upload_prepare → 逐片 PUT 预签名 URL
//     → upload_part_finish → /files 合并），覆盖本地图片/文件（meme、send_file 工具）。
func (a *qqOfficialAdapter) uploadMedia(ctx context.Context, openid string, isGroup bool, fileType int, src, fileName string) (string, error) {
	base := scopePath(isGroup) + "/" + openid
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return a.uploadByURL(ctx, base, fileType, src)
	}
	data, ok := resolveSegmentBytes(src)
	if !ok {
		return "", fmt.Errorf("无法解析的媒体资源源（仅支持 http(s) URL / base64:// / data: / file://）")
	}
	return a.uploadByBytes(ctx, base, fileType, data, fileName)
}

// uploadByURL URL 直传：POST /files {file_type, url, srv_send_msg:false}。
func (a *qqOfficialAdapter) uploadByURL(ctx context.Context, base string, fileType int, url string) (string, error) {
	var res uploadFileResponse
	req := uploadFileRequest{FileType: fileType, URL: url, SrvSendMsg: false}
	if err := a.client.post(ctx, base+"/files", req, &res); err != nil {
		return "", err
	}
	if res.FileInfo == "" {
		return "", fmt.Errorf("上传响应缺少 file_info")
	}
	return res.FileInfo, nil
}

// uploadByBytes 分片上传本地字节内容：
//  1. upload_prepare 获取 upload_id 与各分片预签名 URL（携带整文件 md5/sha1/md5_10m 校验值）
//  2. 逐片 HTTP PUT 到预签名 URL（无需鉴权头），成功后 upload_part_finish 通知
//  3. /files 携带 upload_id 完成合并，返回 file_info
func (a *qqOfficialAdapter) uploadByBytes(ctx context.Context, base string, fileType int, data []byte, fileName string) (string, error) {
	sum := md5.Sum(data)
	sha := sha1.Sum(data)
	window := data
	if len(window) > md510mWindow {
		window = window[:md510mWindow]
	}
	sum10m := md5.Sum(window)
	prepare := uploadPrepareRequest{
		FileType: fileType,
		FileSize: strconv.Itoa(len(data)),
		FileName: fileName,
		Md5:      hex.EncodeToString(sum[:]),
		Sha1:     hex.EncodeToString(sha[:]),
		Md510m:   hex.EncodeToString(sum10m[:]),
	}
	var pre uploadPrepareResponse
	if err := a.client.post(ctx, base+"/upload_prepare", prepare, &pre); err != nil {
		return "", fmt.Errorf("upload_prepare 失败: %w", err)
	}
	if pre.UploadID == "" || len(pre.Parts) == 0 {
		return "", fmt.Errorf("upload_prepare 响应缺少 upload_id 或分片列表")
	}

	// 逐片上传：预签名 URL 为 COS 直链，无需 Authorization 头（独立 http 客户端）
	putClient := resty.New().SetTimeout(2 * time.Minute)
	offset := 0
	for _, part := range pre.Parts {
		blockSize, err := strconv.Atoi(part.BlockSize)
		if err != nil || blockSize <= 0 {
			return "", fmt.Errorf("分片 %d 的 block_size 非法: %q", part.Index, part.BlockSize)
		}
		end := offset + blockSize
		if end > len(data) {
			end = len(data)
		}
		chunk := data[offset:end]
		if len(chunk) == 0 {
			return "", fmt.Errorf("分片 %d 超出文件范围（offset=%d, size=%d）", part.Index, offset, len(data))
		}
		resp, err := putClient.R().SetContext(ctx).
			SetHeader("Content-Type", "application/octet-stream").
			SetBody(chunk).
			Put(part.PresignedURL)
		if err != nil {
			return "", fmt.Errorf("分片 %d PUT 失败: %w", part.Index, err)
		}
		if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
			return "", fmt.Errorf("分片 %d PUT 返回 http %d", part.Index, resp.StatusCode())
		}
		chunkSum := md5.Sum(chunk)
		finish := uploadPartFinishRequest{
			UploadID:  pre.UploadID,
			PartIndex: part.Index,
			BlockSize: strconv.Itoa(len(chunk)),
			Md5:       hex.EncodeToString(chunkSum[:]),
		}
		if err := a.client.post(ctx, base+"/upload_part_finish", finish, nil); err != nil {
			return "", fmt.Errorf("分片 %d upload_part_finish 失败: %w", part.Index, err)
		}
		offset = end
	}
	if offset != len(data) {
		return "", fmt.Errorf("分片上传不完整：已传 %d / %d 字节", offset, len(data))
	}

	// 合并：携带 upload_id 调 /files 换取 file_info
	var res uploadFileResponse
	merge := uploadFileRequest{FileType: fileType, SrvSendMsg: false, FileName: fileName, UploadID: pre.UploadID}
	if err := a.client.post(ctx, base+"/files", merge, &res); err != nil {
		return "", fmt.Errorf("分片合并失败: %w", err)
	}
	if res.FileInfo == "" {
		return "", fmt.Errorf("分片合并响应缺少 file_info")
	}
	return res.FileInfo, nil
}

// resolveSegmentBytes 从 base64://、data:、file:// 段源解析出字节内容
// （http(s) URL 走 URL 直传，不在此路径）。
func resolveSegmentBytes(src string) ([]byte, bool) {
	switch {
	case strings.HasPrefix(src, "base64://"):
		b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(src, "base64://"))
		return b, err == nil && len(b) > 0
	case strings.HasPrefix(src, "data:"):
		if i := strings.Index(src, ","); i >= 0 {
			b, err := base64.StdEncoding.DecodeString(src[i+1:])
			return b, err == nil && len(b) > 0
		}
	case strings.HasPrefix(src, "file://"):
		b, err := os.ReadFile(strings.TrimPrefix(src, "file://"))
		return b, err == nil && len(b) > 0
	}
	return nil, false
}
