package wechat

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
	"github.com/changerr/dishflow-grok/internal/config"
)

type Client interface {
	Code2Session(ctx context.Context, appid, secret, code string) (openid string, err error)
	GetPhoneNumber(ctx context.Context, appid, secret, code string) (e164, country string, err error)
	JSAPIPrepay(ctx context.Context, cfg PayConfig, openid string, amountCents int64, orderID, description, notifyURL string) (prepayID string, params map[string]string, err error)
	QueryOrder(ctx context.Context, cfg PayConfig, orderID string) (QueryResult, error)
	CloseOrder(ctx context.Context, cfg PayConfig, orderID string) error
	Refund(ctx context.Context, cfg PayConfig, orderID, refundNo string, amountCents int64, reason, notifyURL string) error
	QueryRefund(ctx context.Context, cfg PayConfig, refundNo string) (QueryResult, error)
	MiniProgramCode(ctx context.Context, appid, secret, scene string) ([]byte, error)
}

type PayConfig struct {
	AppID      string
	MchID      string
	SerialNo   string
	APIKey     string
	PrivateKey *rsa.PrivateKey
	PubKeyID   string
	PubKey     *rsa.PublicKey
	Platform   *x509.Certificate
}

type QueryResult struct {
	TradeState string
	Amount     int64
	Currency   string
	AppID      string
	MchID      string
	OrderID    string
	TxnID      string
	EventID    string
}

type DevClient struct{ DevMode bool }

func New(cfg config.Config) Client {
	return NewHTTP(cfg.DevMode)
}

func (c DevClient) Code2Session(_ context.Context, _, _, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", apperr.Validation("code 不能为空")
	}
	return "oid_" + strings.TrimSpace(code), nil
}

func (c DevClient) GetPhoneNumber(_ context.Context, _, _, code string) (string, string, error) {
	code = strings.TrimSpace(code)
	if len(code) == 11 && strings.HasPrefix(code, "1") {
		return "+86" + code, "86", nil
	}
	return "+8613800138000", "86", nil
}

func (c DevClient) JSAPIPrepay(_ context.Context, cfg PayConfig, openid string, amountCents int64, orderID, _, _ string) (string, map[string]string, error) {
	prepay := "mock_prepay_" + orderID
	return prepay, map[string]string{
		"appId": cfg.AppID, "timeStamp": fmt.Sprintf("%d", time.Now().Unix()),
		"nonceStr": "mock", "package": "prepay_id=" + prepay, "signType": "RSA", "paySign": "mock",
	}, nil
}

func (c DevClient) QueryOrder(_ context.Context, _ PayConfig, orderID string) (QueryResult, error) {
	return QueryResult{TradeState: "NOTPAY", OrderID: orderID, Currency: "CNY"}, nil
}

func (c DevClient) CloseOrder(context.Context, PayConfig, string) error { return nil }

func (c DevClient) Refund(context.Context, PayConfig, string, string, int64, string, string) error {
	return nil
}

func (c DevClient) QueryRefund(_ context.Context, _ PayConfig, refundNo string) (QueryResult, error) {
	return QueryResult{TradeState: "PROCESSING", OrderID: refundNo}, nil
}

func (c DevClient) MiniProgramCode(_ context.Context, _, _, scene string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			if (x/8+y/8)%2 == 0 {
				img.Set(x, y, color.Black)
			} else {
				img.Set(x, y, color.White)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	_ = scene
	return buf.Bytes(), nil
}

func DecryptNotify(apiV3Key string, nonce, ciphertext, associatedData string) ([]byte, error) {
	key := []byte(apiV3Key)
	if len(key) != 32 {
		return nil, errors.New("invalid apiv3 key")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, []byte(nonce), raw, []byte(associatedData))
}

func EncryptNotify(apiV3Key, nonce, associatedData string, plaintext []byte) (string, error) {
	block, err := aes.NewCipher([]byte(apiV3Key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	out := gcm.Seal(nil, []byte(nonce), plaintext, []byte(associatedData))
	return base64.StdEncoding.EncodeToString(out), nil
}

func VerifyTimestamp(ts string, now time.Time, window time.Duration) bool {
	var sec int64
	_, err := fmt.Sscan(ts, &sec)
	if err != nil {
		return false
	}
	t := time.Unix(sec, 0).UTC()
	delta := now.Sub(t)
	if delta < 0 {
		delta = -delta
	}
	return delta <= window
}

func ParsePrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid pem")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not rsa")
		}
		return rk, nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func ParsePublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid pem")
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rk, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not rsa")
		}
		return rk, nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not rsa")
	}
	return rk, nil
}

func SignSHA256(priv *rsa.PrivateKey, message []byte) (string, error) {
	if priv == nil {
		return "", errors.New("missing private key")
	}
	sum := sha256.Sum256(message)
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func VerifySHA256(pub *rsa.PublicKey, message []byte, sigB64 string) error {
	if pub == nil {
		return errors.New("missing public key")
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(message)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig)
}

func NotifyMessage(ts, nonce, body string) []byte {
	return []byte(ts + "\n" + nonce + "\n" + body + "\n")
}

func SafeHTTPDo(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return http.DefaultClient.Do(req)
}

func JSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
