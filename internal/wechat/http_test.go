package wechat_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/changerr/dishflow-grok/internal/wechat"
)

func TestSignAndVerifyNotify(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"id":"evt1"}`
	ts, nonce := "1700000000", "n1"
	msg := wechat.NotifyMessage(ts, nonce, body)
	sig, err := wechat.SignSHA256(priv, msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := wechat.VerifySHA256(&priv.PublicKey, msg, sig); err != nil {
		t.Fatal(err)
	}
	if err := wechat.VerifySHA256(&priv.PublicKey, []byte("tampered"), sig); err == nil {
		t.Fatal("tampered body must fail")
	}
}

func TestJSAPIPrepaySignsPayParams(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/pay/transactions/jsapi" {
			t.Errorf("path %s", r.URL.Path)
		}
		if !strings.Contains(r.Header.Get("Authorization"), "WECHATPAY2-SHA256-RSA2048") {
			t.Error("missing auth")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"prepay_id": "wx123"})
	}))
	defer srv.Close()
	c := wechat.NewHTTP(false)
	c.HTTP = srv.Client()
	c.PayBase = srv.URL
	cfg := wechat.PayConfig{AppID: "wxapp", MchID: "mch", SerialNo: "ser", PrivateKey: priv}
	id, params, err := c.JSAPIPrepay(context.Background(), cfg, "oid", 100, "ord1", "DishFlow", srv.URL+"/cb")
	if err != nil {
		t.Fatal(err)
	}
	if id != "wx123" || params["signType"] != "RSA" || params["paySign"] == "" || params["package"] != "prepay_id=wx123" {
		t.Fatalf("%s %#v", id, params)
	}
}

func TestQueryOrderParsesTradeState(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"trade_state":"SUCCESS","appid":"wx","mchid":"m","out_trade_no":"o1","transaction_id":"t1","amount":{"total":2800,"currency":"CNY"}}`)
	}))
	defer srv.Close()
	c := wechat.NewHTTP(false)
	c.HTTP = srv.Client()
	c.PayBase = srv.URL
	got, err := c.QueryOrder(context.Background(), wechat.PayConfig{MchID: "m", SerialNo: "s", PrivateKey: priv}, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if got.TradeState != "SUCCESS" || got.Amount != 2800 || got.TxnID != "t1" {
		t.Fatalf("%+v", got)
	}
}
