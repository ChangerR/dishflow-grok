package app

import (
	"context"
	"testing"

	"github.com/changerr/dishflow-grok/internal/apperr"
)

func TestCheckLoginLimitSkippedWithoutRedis(t *testing.T) {
	a := &App{}
	if err := a.checkLoginLimit(context.Background(), "127.0.0.1", "owner1"); err != nil {
		t.Fatal(err)
	}
	a.recordLoginFailure(context.Background(), "127.0.0.1", "owner1")
	a.clearLoginFailures(context.Background(), "owner1")
}

func TestLoginLimitKeys(t *testing.T) {
	keys := loginLimitKeys("127.0.0.1", "owner1")
	if keys[0] != "login:ip:127.0.0.1" || keys[1] != "login:user:owner1" {
		t.Fatalf("got %v", keys)
	}
	if loginFailureLimit < 1 {
		t.Fatal("limit")
	}
	if apperr.RateLimited == nil {
		t.Fatal("rate limited sentinel")
	}
}
