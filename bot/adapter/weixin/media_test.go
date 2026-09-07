package weixin

import (
	"bytes"
	"crypto/aes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestEncryptDecryptAESECBRoundTrip(t *testing.T) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	cases := [][]byte{
		[]byte(""),
		[]byte("a"),
		[]byte("0123456789abcdef"),      // 恰好整块（PK7 补整块）
		bytes.Repeat([]byte("中文"), 100), // 多字节字符
		make([]byte, 4096),
	}
	for i, pt := range cases {
		ct, err := encryptAESECB(pt, key)
		if err != nil {
			t.Fatalf("case %d: encrypt: %v", i, err)
		}
		if len(ct) != aesPaddedSize(len(pt)) {
			t.Fatalf("case %d: ciphertext size %d, want %d", i, len(ct), aesPaddedSize(len(pt)))
		}
		got, err := decryptAESECB(ct, key)
		if err != nil {
			t.Fatalf("case %d: decrypt: %v", i, err)
		}
		if !bytes.Equal(got, pt) {
			t.Fatalf("case %d: round-trip mismatch", i)
		}
	}
}

func TestDecryptAESECBWrongKey(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 16)
	other := bytes.Repeat([]byte{2}, 16)
	ct, err := encryptAESECB([]byte("hello weixin"), key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decryptAESECB(ct, other); err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestDecryptAESECBBadLength(t *testing.T) {
	key := bytes.Repeat([]byte{3}, 16)
	if _, err := decryptAESECB([]byte("not block size"), key); err == nil {
		t.Fatal("expected error for non-block-multiple ciphertext")
	}
}

func TestAESEPaddedSize(t *testing.T) {
	cases := map[int]int{
		0:  16,
		1:  16,
		15: 16,
		16: 32,
		17: 32,
		31: 32,
		32: 48,
	}
	for in, want := range cases {
		if got := aesPaddedSize(in); got != want {
			t.Fatalf("aesPaddedSize(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestParseCDNAesKey(t *testing.T) {
	raw := bytes.Repeat([]byte{0xAB}, 16)
	hexStr := "0123456789abcdef0123456789abcdef"

	// 图片：base64(原始 16 字节)
	k, err := parseCDNAesKey(stdB64(t, raw))
	if err != nil || !bytes.Equal(k, raw) {
		t.Fatalf("raw-16 key: k=%v err=%v", k, err)
	}
	// 文件/语音/视频：base64(32 位 hex 串)
	k, err = parseCDNAesKey(stdB64(t, []byte(hexStr)))
	if err != nil {
		t.Fatalf("hex key: %v", err)
	}
	if want, _ := hexDecode(hexStr); !bytes.Equal(k, want) {
		t.Fatalf("hex key mismatch: got %v", k)
	}
	// 非法长度
	if _, err := parseCDNAesKey(stdB64(t, []byte("short"))); err == nil {
		t.Fatal("expected error for invalid key length")
	}
	// 非 base64
	if _, err := parseCDNAesKey("!!!not-base64!!!"); err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestAESCipherBlockSize(t *testing.T) {
	if aes.BlockSize != 16 {
		t.Fatalf("protocol assumes AES-128 block size 16, got %d", aes.BlockSize)
	}
}

func stdB64(t *testing.T, b []byte) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(b)
}

func hexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}
