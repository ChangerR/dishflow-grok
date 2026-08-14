package wechat_test

import (
	"testing"
	"time"

	"github.com/changerr/dishflow-grok/internal/wechat"
)

func TestNotifyDecryptRoundTripAndTimestampWindow(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	nonce := "123456789012"
	ad := "transaction"
	plain := []byte(`{"out_trade_no":"o1","amount":{"total":100}}`)
	ct, err := wechat.EncryptNotify(key, nonce, ad, plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := wechat.DecryptNotify(key, nonce, ct, ad)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(plain) {
		t.Fatalf("got %s", got)
	}
	now := time.Unix(1_700_000_000, 0).UTC()
	if !wechat.VerifyTimestamp("1700000000", now, 5*time.Minute) {
		t.Fatal("in window")
	}
	if wechat.VerifyTimestamp("1699990000", now, 5*time.Minute) {
		t.Fatal("expired should fail")
	}
}
