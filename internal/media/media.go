package media

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/webp"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/ids"
)

type Store interface {
	Put(kind, ext string, data []byte) (key string, url string, err error)
	Get(key string) ([]byte, string, error)
}

type Local struct{ Dir string }

func NewLocal(dir string) Store {
	_ = os.MkdirAll(dir, 0o755)
	return Local{Dir: dir}
}

func (l Local) Put(kind, ext string, data []byte) (string, string, error) {
	if err := ValidateImage(data); err != nil {
		return "", "", err
	}
	key := filepath.ToSlash(filepath.Join(kind, ids.New()+ext))
	full := filepath.Join(l.Dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", "", err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", "", err
	}
	return key, "/media/menu/" + key, nil
}

func (l Local) Get(key string) ([]byte, string, error) {
	key = strings.TrimPrefix(key, "/")
	if strings.Contains(key, "..") {
		return nil, "", apperr.NotFound
	}
	full := filepath.Join(l.Dir, filepath.FromSlash(key))
	b, err := os.ReadFile(full)
	if err != nil {
		return nil, "", apperr.NotFound
	}
	ct := "application/octet-stream"
	switch strings.ToLower(filepath.Ext(key)) {
	case ".jpg", ".jpeg":
		ct = "image/jpeg"
	case ".png":
		ct = "image/png"
	case ".webp":
		ct = "image/webp"
	}
	return b, ct, nil
}

func ValidateImage(data []byte) error {
	if len(data) > 2*1024*1024 {
		return apperr.Validation("图片不能超过 2MB")
	}
	r := bytes.NewReader(data)
	cfg, format, err := image.DecodeConfig(r)
	if err != nil {
		if _, err2 := webp.DecodeConfig(bytes.NewReader(data)); err2 != nil {
			return apperr.Validation("仅支持 JPEG/PNG/WebP")
		}
		cfg, format, err = decodeWebpConfig(data)
		if err != nil {
			return apperr.Validation("无法解码图片")
		}
	}
	_ = format
	if cfg.Width > 4096 || cfg.Height > 4096 {
		return apperr.Validation("图片尺寸不能超过 4096×4096")
	}
	if cfg.Width*cfg.Height > 16_000_000 {
		return apperr.Validation("图片像素不能超过 1600 万")
	}
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		if _, err2 := webp.Decode(bytes.NewReader(data)); err2 != nil {
			return apperr.Validation("图片解码失败")
		}
	}
	return nil
}

func decodeWebpConfig(data []byte) (image.Config, string, error) {
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	return cfg, "webp", err
}

func ReadAll(r io.Reader, max int64) ([]byte, error) {
	var buf bytes.Buffer
	n, err := io.Copy(&buf, io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if n > max {
		return nil, fmt.Errorf("too large")
	}
	return buf.Bytes(), nil
}

func TrustedImageURL(u string) bool {
	u = strings.ToLower(strings.TrimSpace(u))
	if u == "" {
		return true
	}
	if strings.HasPrefix(u, "/media/") {
		return true
	}
	if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
		if strings.Contains(u, "javascript:") || strings.HasPrefix(u, "data:") {
			return false
		}
		return true
	}
	return false
}
