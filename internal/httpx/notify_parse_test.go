package httpx

import (
	"encoding/json"
	"testing"

	"github.com/changerr/dishflow-grok/internal/wechat"
)

func TestParsePayNotifyDecryptAndRejectTamper(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	plain := []byte(`{"appid":"wx","mchid":"m","out_trade_no":"o1","transaction_id":"t1","trade_state":"SUCCESS","amount":{"total":100,"currency":"CNY"}}`)
	ct, err := wechat.EncryptNotify(key, "123456789012", "transaction", plain)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{
		"id": "evt1",
		"resource": map[string]string{"ciphertext": ct, "nonce": "123456789012", "associated_data": "transaction"},
	})
	eventID, appid, mchid, orderID, txnID, amount, currency, trade, err := parsePayNotify(body, key)
	if err != nil {
		t.Fatal(err)
	}
	if eventID != "evt1" || appid != "wx" || mchid != "m" || orderID != "o1" || txnID != "t1" || amount != 100 || currency != "CNY" || trade != "SUCCESS" {
		t.Fatalf("%s %s %s %s %s %d %s %s", eventID, appid, mchid, orderID, txnID, amount, currency, trade)
	}
	_, _, _, _, _, _, _, _, err = parsePayNotify(body, "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	if err == nil {
		t.Fatal("tampered key must fail decrypt")
	}
}
