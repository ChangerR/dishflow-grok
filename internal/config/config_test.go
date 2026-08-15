package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDotEnvKeepsUTF8Chinese(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	body := "BOOTSTRAP_PLATFORM_NAME=平台管理员\nSHOP_ENV=development\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := parseDotEnv(p)
	if err != nil {
		t.Fatal(err)
	}
	if m["BOOTSTRAP_PLATFORM_NAME"] != "平台管理员" {
		t.Fatalf("got %q", m["BOOTSTRAP_PLATFORM_NAME"])
	}
}

func TestIsGBKMojibake(t *testing.T) {
	if !isGBKMojibake("骞冲彴绠＄悊鍛?", "平台管理员") {
		t.Fatal("expected mojibake of 平台管理员 to be detected")
	}
	if isGBKMojibake("平台管理员", "平台管理员") {
		t.Fatal("correct name is not mojibake")
	}
}

func TestProductionRejectsShortSecrets(t *testing.T) {
	t.Setenv("SHOP_ENV", "production")
	t.Setenv("SHOP_SESSION_SECRET", "short")
	t.Setenv("SHOP_QUOTE_SECRET", "also-short")
	t.Setenv("SHOP_CREDENTIAL_KEY", "0123456789abcdef0123456789abcdef")
	_, err := Load()
	if err == nil {
		t.Fatal("expected production validation error")
	}
	_ = os.Unsetenv("SHOP_ENV")
}
