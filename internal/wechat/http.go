package wechat

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/changerr/dishflow-grok/internal/apperr"
)

type HTTPClient struct {
	HTTP    *http.Client
	DevMode bool
	PayBase string
	WXBase  string
	tokens  sync.Map
}

func NewHTTP(dev bool) *HTTPClient {
	return &HTTPClient{
		HTTP:    &http.Client{Timeout: 15 * time.Second},
		DevMode: dev,
		PayBase: "https://api.mch.weixin.qq.com",
		WXBase:  "https://api.weixin.qq.com",
	}
}

func (c *HTTPClient) fallback() DevClient { return DevClient{DevMode: c.DevMode} }

func (c *HTTPClient) Code2Session(ctx context.Context, appid, secret, code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", apperr.Validation("code 不能为空")
	}
	if secret == "" {
		if c.DevMode {
			return c.fallback().Code2Session(ctx, appid, secret, code)
		}
		return "", apperr.New(409, "WECHAT_LOGIN_UNAVAILABLE", "门店未配置小程序密钥")
	}
	q := url.Values{
		"appid": {appid}, "secret": {secret}, "js_code": {code}, "grant_type": {"authorization_code"},
	}
	var out struct {
		OpenID string `json:"openid"`
		ErrMsg string `json:"errmsg"`
		Err    int    `json:"errcode"`
	}
	if err := c.getJSON(ctx, c.WXBase+"/sns/jscode2session?"+q.Encode(), &out); err != nil {
		return "", err
	}
	if out.Err != 0 || out.OpenID == "" {
		return "", apperr.New(502, "WECHAT_LOGIN_UNAVAILABLE", "微信登录失败")
	}
	return out.OpenID, nil
}

func (c *HTTPClient) GetPhoneNumber(ctx context.Context, appid, secret, code string) (string, string, error) {
	if secret == "" {
		if c.DevMode {
			return c.fallback().GetPhoneNumber(ctx, appid, secret, code)
		}
		return "", "", apperr.New(409, "WECHAT_LOGIN_UNAVAILABLE", "门店未配置小程序密钥")
	}
	tok, err := c.accessToken(ctx, appid, secret)
	if err != nil {
		return "", "", err
	}
	body := JSON(map[string]string{"code": code})
	var out struct {
		Err    int    `json:"errcode"`
		Phone  struct {
			PurePhone string `json:"purePhoneNumber"`
			Country   string `json:"countryCode"`
		} `json:"phone_info"`
	}
	if err := c.postJSON(ctx, c.WXBase+"/wxa/business/getuserphonenumber?access_token="+url.QueryEscape(tok), body, nil, &out); err != nil {
		return "", "", err
	}
	if out.Err != 0 || out.Phone.PurePhone == "" {
		return "", "", apperr.Validation("无法验证手机号")
	}
	cc := out.Phone.Country
	if cc == "" {
		cc = "86"
	}
	return "+" + cc + out.Phone.PurePhone, cc, nil
}

func (c *HTTPClient) JSAPIPrepay(ctx context.Context, cfg PayConfig, openid string, amountCents int64, orderID, description, notifyURL string) (string, map[string]string, error) {
	if cfg.PrivateKey == nil {
		if c.DevMode {
			return c.fallback().JSAPIPrepay(ctx, cfg, openid, amountCents, orderID, description, notifyURL)
		}
		return "", nil, apperr.PaymentUnavailable
	}
	payload := map[string]any{
		"appid": cfg.AppID, "mchid": cfg.MchID, "description": description,
		"out_trade_no": orderID, "notify_url": notifyURL,
		"amount": map[string]any{"total": amountCents, "currency": "CNY"},
		"payer":  map[string]string{"openid": openid},
	}
	raw := JSON(payload)
	var out struct {
		PrepayID string `json:"prepay_id"`
		Message  string `json:"message"`
	}
	if err := c.payJSON(ctx, cfg, http.MethodPost, "/v3/pay/transactions/jsapi", raw, &out); err != nil {
		return "", nil, err
	}
	if out.PrepayID == "" {
		return "", nil, apperr.PaymentUnavailable
	}
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce, _ := randomHex(16)
	pkg := "prepay_id=" + out.PrepayID
	msg := cfg.AppID + "\n" + ts + "\n" + nonce + "\n" + pkg + "\n"
	sig, err := SignSHA256(cfg.PrivateKey, []byte(msg))
	if err != nil {
		return "", nil, err
	}
	return out.PrepayID, map[string]string{
		"appId": cfg.AppID, "timeStamp": ts, "nonceStr": nonce,
		"package": pkg, "signType": "RSA", "paySign": sig,
	}, nil
}

func (c *HTTPClient) QueryOrder(ctx context.Context, cfg PayConfig, orderID string) (QueryResult, error) {
	if cfg.PrivateKey == nil {
		if c.DevMode {
			return c.fallback().QueryOrder(ctx, cfg, orderID)
		}
		return QueryResult{}, apperr.PaymentUnavailable
	}
	path := "/v3/pay/transactions/out-trade-no/" + url.PathEscape(orderID) + "?mchid=" + url.QueryEscape(cfg.MchID)
	var out struct {
		TradeState    string `json:"trade_state"`
		Appid         string `json:"appid"`
		Mchid         string `json:"mchid"`
		OutTradeNo    string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`
		Amount        struct {
			Total    int64  `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if err := c.payJSON(ctx, cfg, http.MethodGet, path, nil, &out); err != nil {
		return QueryResult{}, err
	}
	return QueryResult{
		TradeState: out.TradeState, Amount: out.Amount.Total, Currency: out.Amount.Currency,
		AppID: out.Appid, MchID: out.Mchid, OrderID: out.OutTradeNo, TxnID: out.TransactionID,
	}, nil
}

func (c *HTTPClient) CloseOrder(ctx context.Context, cfg PayConfig, orderID string) error {
	if cfg.PrivateKey == nil {
		if c.DevMode {
			return c.fallback().CloseOrder(ctx, cfg, orderID)
		}
		return apperr.PaymentUnavailable
	}
	path := "/v3/pay/transactions/out-trade-no/" + url.PathEscape(orderID) + "/close"
	return c.payJSON(ctx, cfg, http.MethodPost, path, JSON(map[string]string{"mchid": cfg.MchID}), &map[string]any{})
}

func (c *HTTPClient) Refund(ctx context.Context, cfg PayConfig, orderID, refundNo string, amountCents int64, reason, notifyURL string) error {
	if cfg.PrivateKey == nil {
		if c.DevMode {
			return c.fallback().Refund(ctx, cfg, orderID, refundNo, amountCents, reason, notifyURL)
		}
		return apperr.PaymentUnavailable
	}
	payload := map[string]any{
		"out_trade_no": orderID, "out_refund_no": refundNo, "reason": reason, "notify_url": notifyURL,
		"amount": map[string]any{"refund": amountCents, "total": amountCents, "currency": "CNY"},
	}
	return c.payJSON(ctx, cfg, http.MethodPost, "/v3/refund/domestic/refunds", JSON(payload), &map[string]any{})
}

func (c *HTTPClient) QueryRefund(ctx context.Context, cfg PayConfig, refundNo string) (QueryResult, error) {
	if cfg.PrivateKey == nil {
		if c.DevMode {
			return c.fallback().QueryRefund(ctx, cfg, refundNo)
		}
		return QueryResult{}, apperr.PaymentUnavailable
	}
	path := "/v3/refund/domestic/refunds/" + url.PathEscape(refundNo)
	var out struct {
		Status        string `json:"status"`
		OutTradeNo    string `json:"out_trade_no"`
		OutRefundNo   string `json:"out_refund_no"`
		RefundID      string `json:"refund_id"`
		Amount        struct {
			Refund   int64  `json:"refund"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if err := c.payJSON(ctx, cfg, http.MethodGet, path, nil, &out); err != nil {
		return QueryResult{}, err
	}
	return QueryResult{
		TradeState: out.Status, Amount: out.Amount.Refund, Currency: out.Amount.Currency,
		OrderID: out.OutTradeNo, TxnID: out.RefundID, EventID: out.OutRefundNo,
	}, nil
}

func (c *HTTPClient) MiniProgramCode(ctx context.Context, appid, secret, scene string) ([]byte, error) {
	if secret == "" {
		if c.DevMode {
			return c.fallback().MiniProgramCode(ctx, appid, secret, scene)
		}
		return nil, apperr.New(409, "WECHAT_LOGIN_UNAVAILABLE", "门店未配置小程序密钥")
	}
	tok, err := c.accessToken(ctx, appid, secret)
	if err != nil {
		return nil, err
	}
	raw := JSON(map[string]any{"scene": scene, "check_path": false, "env_version": "release"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.WXBase+"/wxa/getwxacodeunlimit?access_token="+url.QueryEscape(tok), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if len(b) > 8 && b[0] == '{' {
		return nil, apperr.New(502, "WECHAT_LOGIN_UNAVAILABLE", "无法生成小程序码")
	}
	return b, nil
}

func (c *HTTPClient) accessToken(ctx context.Context, appid, secret string) (string, error) {
	if v, ok := c.tokens.Load(appid); ok {
		if t, ok := v.(cachedToken); ok && time.Now().Before(t.exp) {
			return t.val, nil
		}
	}
	q := url.Values{"grant_type": {"client_credential"}, "appid": {appid}, "secret": {secret}}
	var out struct {
		Token string `json:"access_token"`
		Exp   int    `json:"expires_in"`
		Err   int    `json:"errcode"`
	}
	if err := c.getJSON(ctx, c.WXBase+"/cgi-bin/token?"+q.Encode(), &out); err != nil {
		return "", err
	}
	if out.Err != 0 || out.Token == "" {
		return "", apperr.New(502, "WECHAT_LOGIN_UNAVAILABLE", "无法获取 access_token")
	}
	ttl := time.Duration(out.Exp) * time.Second
	if ttl > 2*time.Minute {
		ttl -= time.Minute
	}
	c.tokens.Store(appid, cachedToken{val: out.Token, exp: time.Now().Add(ttl)})
	return out.Token, nil
}

type cachedToken struct {
	val string
	exp time.Time
}

func (c *HTTPClient) payJSON(ctx context.Context, cfg PayConfig, method, path string, body []byte, out any) error {
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}
	bodyStr := ""
	if body != nil {
		bodyStr = string(body)
	}
	u, err := url.Parse(c.PayBase + strings.Split(path, "?")[0])
	if err != nil {
		return err
	}
	signPath := u.EscapedPath()
	if q := strings.SplitN(path, "?", 2); len(q) == 2 {
		signPath += "?" + q[1]
	}
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce, _ := randomHex(16)
	msg := method + "\n" + signPath + "\n" + ts + "\n" + nonce + "\n" + bodyStr + "\n"
	sig, err := SignSHA256(cfg.PrivateKey, []byte(msg))
	if err != nil {
		return err
	}
	auth := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`,
		cfg.MchID, nonce, ts, cfg.SerialNo, sig)
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.PayBase+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return apperr.New(502, "PAYMENT_UNAVAILABLE", "微信支付接口失败")
	}
	if out == nil || len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, out)
}

func (c *HTTPClient) getJSON(ctx context.Context, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func (c *HTTPClient) postJSON(ctx context.Context, rawURL string, body []byte, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
