// Package httpx is the small HTTP helper the source layer shares.
package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// TVBoxUA is the client user agent many CMS endpoints expect.
const TVBoxUA = "okhttp/3.12.11"

// BrowserUA is used for HTML pages behind rule and drpy sources.
const BrowserUA = "Mozilla/5.0 (Linux; Android 11; Pixel 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36"

// New returns a client with a bounded timeout. Callers can also pass a
// context for a shorter deadline.
func New() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// Get issues a GET and returns the body and Content-Type. file: URLs are
// read from disk. Bodies larger than limit are rejected.
func Get(ctx context.Context, hc *http.Client, raw string, headers map[string]string, limit int64) ([]byte, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 8 << 20
	}
	if strings.HasPrefix(raw, "file:") {
		b, err := readFileURL(raw, limit)
		return b, "", err
	}
	if hc == nil {
		hc = New()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", TVBoxUA)
	for k, v := range headers {
		if strings.TrimSpace(k) == "" || v == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(body)) > limit {
		return nil, "", fmt.Errorf("响应超过 %d 字节", limit)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.Header.Get("Content-Type"), fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	return ToUTF8(body, ct), ct, nil
}

// PostForm posts an application/x-www-form-urlencoded body.
func PostForm(ctx context.Context, hc *http.Client, raw, form string, headers map[string]string, limit int64) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if hc == nil {
		hc = New()
	}
	if limit <= 0 {
		limit = 8 << 20
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, raw, strings.NewReader(form))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", BrowserUA)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range headers {
		if strings.TrimSpace(k) != "" && v != "" {
			req.Header.Set(k, v)
		}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("响应超过 %d 字节", limit)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return ToUTF8(body, resp.Header.Get("Content-Type")), nil
}

func readFileURL(raw string, limit int64) ([]byte, error) {
	path := strings.TrimPrefix(raw, "file://")
	path = strings.TrimPrefix(path, "file:")
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("文件超过 %d 字节", limit)
	}
	return b, nil
}

// ToUTF8 converts GBK/GB18030 text bodies. Images, video and other
// binary payloads are returned unchanged: PNG and JPEG are not valid
// UTF-8, and decoding them as GB18030 corrupts the file.
func ToUTF8(b []byte, contentType string) []byte {
	if len(b) == 0 || isBinary(b, contentType) {
		return b
	}
	hint := strings.ToLower(contentType)
	head := strings.ToLower(string(b[:min(180, len(b))]))
	need := strings.Contains(hint, "gbk") || strings.Contains(hint, "gb2312") || strings.Contains(hint, "gb18030") ||
		strings.Contains(head, "charset=gbk") || strings.Contains(head, "charset=gb2312") || strings.Contains(head, "charset=gb18030") ||
		strings.Contains(head, "encoding=\"gbk\"") || strings.Contains(head, "encoding=\"gb2312\"")
	if !need && utf8.Valid(b) {
		return b
	}
	out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	if err != nil || len(out) == 0 {
		return b
	}
	return out
}

func isBinary(b []byte, contentType string) bool {
	ct := strings.ToLower(contentType)
	for _, part := range []string{"image/", "video/", "audio/", "octet-stream", "font/"} {
		if strings.Contains(ct, part) {
			return true
		}
	}
	if len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return true
	}
	if len(b) >= 3 && b[0] == 0xff && b[1] == 0xd8 && b[2] == 0xff {
		return true
	}
	if len(b) >= 6 && (bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a"))) {
		return true
	}
	if len(b) >= 12 && string(b[4:8]) == "ftyp" {
		return true
	}
	return false
}
