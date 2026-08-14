package config_test

import (
	"os"
	"testing"

	"github.com/changerr/dishflow-grok/internal/config"
)

func TestProductionRejectsShortSecrets(t *testing.T) {
	t.Setenv("SHOP_ENV", "production")
	t.Setenv("SHOP_SESSION_SECRET", "short")
	t.Setenv("SHOP_QUOTE_SECRET", "also-short")
	t.Setenv("SHOP_CREDENTIAL_KEY", "0123456789abcdef0123456789abcdef")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected production validation error")
	}
	_ = os.Unsetenv("SHOP_ENV")
}
