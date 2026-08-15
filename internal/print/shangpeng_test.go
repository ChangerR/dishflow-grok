package print_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/changerr/dishflow-grok/internal/print"
)

func TestShangpengPrintPostsHMAC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/print" {
			t.Errorf("path %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"sn":"SN1"`) || !strings.Contains(string(b), `"sign"`) {
			t.Fatalf("body %s", b)
		}
		_, _ = w.Write([]byte(`{"id":"job9"}`))
	}))
	defer srv.Close()
	cli := print.ShangpengClient{HTTP: srv.Client()}
	id, err := cli.Print(context.Background(), print.ShangpengConfig{AppID: "a", AppSecret: "s", SN: "SN1", BaseURL: srv.URL}, "hello", "idem1")
	if err != nil || id != "job9" {
		t.Fatalf("%s %v", id, err)
	}
}
