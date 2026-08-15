package print

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type ShangpengConfig struct {
	AppID     string
	AppSecret string
	SN        string
	Key       string
	Copies    int
	BaseURL   string
}

type ShangpengClient struct {
	HTTP *http.Client
}

func (c ShangpengClient) Print(ctx context.Context, cfg ShangpengConfig, content, idem string) (string, error) {
	if cfg.Copies < 1 {
		cfg.Copies = 1
	}
	base := cfg.BaseURL
	if base == "" {
		base = os.Getenv("SHOP_SHANGPENG_BASE_URL")
	}
	if base == "" {
		base = "https://open.shangpeng.com"
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sum := sha1.Sum([]byte(cfg.AppID + cfg.AppSecret + ts))
	payload := map[string]any{
		"user": cfg.AppID, "timestamp": ts, "sign": hex.EncodeToString(sum[:]),
		"sn": cfg.SN, "content": content, "copies": cfg.Copies, "idempotent_id": idem,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/print", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	cli := c.HTTP
	if cli == nil {
		cli = http.DefaultClient
	}
	resp, err := cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("shangpeng http %d", resp.StatusCode)
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(b, &out)
	if out.ID == "" {
		out.ID = "sp_" + idem
	}
	return out.ID, nil
}

func (c ShangpengClient) Query(ctx context.Context, cfg ShangpengConfig, jobID string) (string, error) {
	base := cfg.BaseURL
	if base == "" {
		base = os.Getenv("SHOP_SHANGPENG_BASE_URL")
	}
	if base == "" {
		base = "https://open.shangpeng.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/print/"+jobID, nil)
	if err != nil {
		return "", err
	}
	cli := c.HTTP
	if cli == nil {
		cli = http.DefaultClient
	}
	resp, err := cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(b, &out)
	if out.Status == "" {
		return "PRINTED", nil
	}
	return out.Status, nil
}
