package weixin

import (
	"context"
	"crypto/aes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// 媒体经微信 CDN 中转，载荷统一 AES-128-ECB + PKCS7 加密（参考 openclaw-weixin）。

// encryptAESECB AES-128-ECB + PKCS7 加密。
func encryptAESECB(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	pad := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+pad)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(pad)
	}
	ct := make([]byte, len(padded))
	for off := 0; off < len(padded); off += aes.BlockSize {
		block.Encrypt(ct[off:off+aes.BlockSize], padded[off:off+aes.BlockSize])
	}
	return ct, nil
}

// decryptAESECB AES-128-ECB + PKCS7 解密。
func decryptAESECB(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("weixin aes: 密文长度 %d 不是块大小的整数倍", len(ciphertext))
	}
	pt := make([]byte, len(ciphertext))
	for off := 0; off < len(ciphertext); off += aes.BlockSize {
		block.Decrypt(pt[off:off+aes.BlockSize], ciphertext[off:off+aes.BlockSize])
	}
	pad := int(pt[len(pt)-1])
	if pad <= 0 || pad > aes.BlockSize || pad > len(pt) {
		return nil, fmt.Errorf("weixin aes: 非法 PKCS7 填充 %d", pad)
	}
	// 校验填充字节一致，密钥错误时在此暴露而非输出乱码
	padBytes := pt[len(pt)-pad:]
	good := 1
	for _, b := range padBytes {
		good &= subtle.ConstantTimeByteEq(b, byte(pad))
	}
	if good != 1 {
		return nil, fmt.Errorf("weixin aes: PKCS7 填充校验失败（密钥不匹配或数据损坏）")
	}
	return pt[:len(pt)-pad], nil
}

var hexKeyRe = regexp.MustCompile(`^[0-9a-fA-F]{32}$`)

// parseCDNAesKey 解析 CDNMedia.aes_key。线上存在两种编码：
//   - base64(原始 16 字节)：图片（media 字段内 aes_key）
//   - base64(32 位 hex 串)：文件/语音/视频 —— base64 解码后得到 32 个 ASCII hex 字符，需再按 hex 解析
func parseCDNAesKey(aesKeyBase64 string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(aesKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("weixin aes_key base64 解码失败: %w", err)
	}
	switch {
	case len(decoded) == 16:
		return decoded, nil
	case len(decoded) == 32 && hexKeyRe.Match(decoded):
		return hex.DecodeString(string(decoded))
	}
	return nil, fmt.Errorf("weixin aes_key 须为 16 字节原始 key 或 32 位 hex 串的 base64，实际 %d 字节", len(decoded))
}

// uploadedMedia CDN 上传结果。
type uploadedMedia struct {
	Filekey string
	// DownloadParam 上传后 CDN 返回的下载加密参数（填入 CDNMedia.EncryptQueryParam）
	DownloadParam string
	// AeskeyHex 本次上传使用的 AES key（hex）
	AeskeyHex string
	// RawSize 明文大小；CipherSize 密文大小
	RawSize    int64
	CipherSize int64
}

// uploadMedia 通用上传管线：随机 key → getuploadurl → AES-ECB 加密 → POST CDN。
// toUserID 为收件人原始 ID；mediaType 见 UploadMedia* 常量。
func (a *weixinAdapter) uploadMedia(ctx context.Context, data []byte, toUserID string, mediaType int) (*uploadedMedia, error) {
	rawKey := make([]byte, 16)
	if _, err := rand.Read(rawKey); err != nil {
		return nil, err
	}
	filekey := make([]byte, 16)
	if _, err := rand.Read(filekey); err != nil {
		return nil, err
	}
	keyHex := hex.EncodeToString(rawKey)
	fkHex := hex.EncodeToString(filekey)

	resp, err := a.client.getUploadURL(ctx, &GetUploadUrlReq{
		Filekey:     fkHex,
		MediaType:   mediaType,
		ToUserID:    toUserID,
		Rawsize:     int64(len(data)),
		Rawfilemd5:  md5Hex(data),
		Filesize:    int64(aesPaddedSize(len(data))),
		NoNeedThumb: true,
		Aeskey:      keyHex,
	})
	if err != nil {
		return nil, err
	}
	uploadURL := strings.TrimSpace(resp.UploadFullURL)
	if uploadURL == "" && resp.UploadParam != "" {
		uploadURL = a.client.cdnBase + "/upload?encrypted_query_param=" + queryEscape(resp.UploadParam) + "&filekey=" + queryEscape(fkHex)
	}
	if uploadURL == "" {
		return nil, fmt.Errorf("weixin getuploadurl: 未返回上传地址")
	}
	ciphertext, err := encryptAESECB(data, rawKey)
	if err != nil {
		return nil, err
	}
	downloadParam, err := a.client.upload(ctx, uploadURL, ciphertext)
	if err != nil {
		return nil, err
	}
	return &uploadedMedia{
		Filekey:       fkHex,
		DownloadParam: downloadParam,
		AeskeyHex:     keyHex,
		RawSize:       int64(len(data)),
		CipherSize:    int64(len(ciphertext)),
	}, nil
}

// aesPaddedSize 计算 AES-128-ECB PKCS7 后的密文大小（PKCS7 至少补 1 字节，
// 整块时额外补一整块）：等价于 ceil((n+1)/16)*16。
func aesPaddedSize(n int) int {
	return ((n + 16) / 16) * 16
}

// downloadCdnMedia 下载并解密 CDN 媒体。aesKeyB64 为 CDNMedia.AesKey（base64），
// 优先使用 fullURL（服务端直发完整地址），否则按 encryptQueryParam 拼 CDN 下载地址。
func (a *weixinAdapter) downloadCdnMedia(ctx context.Context, media *CDNMedia, aesKeyB64 string) ([]byte, error) {
	if media == nil {
		return nil, fmt.Errorf("weixin cdn: 无媒体引用")
	}
	url := strings.TrimSpace(media.FullURL)
	if url == "" {
		if media.EncryptQueryParam == "" {
			return nil, fmt.Errorf("weixin cdn: 无下载地址（缺少 full_url 与 encrypt_query_param）")
		}
		url = a.client.cdnBase + "/download?encrypted_query_param=" + queryEscape(media.EncryptQueryParam)
	}
	encrypted, err := a.client.download(ctx, url)
	if err != nil {
		return nil, err
	}
	key, err := parseCDNAesKey(aesKeyB64)
	if err != nil {
		return nil, err
	}
	return decryptAESECB(encrypted, key)
}
