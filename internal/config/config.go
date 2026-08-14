package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env               string
	HTTPAddr          string
	PublicBaseURL     string
	MySQLDSN          string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	SessionSecret     string
	QuoteSecret       string
	CredentialKey     []byte
	PhoneHashPepper   string
	TrustedProxies    []string
	DevMode           bool
	AdminDist         string
	UploadDir         string
	CookieSecure      bool
	CookieDomain      string
	COSSecretID       string
	COSSecretKey      string
	COSBucket         string
	COSRegion         string
	COSPrefix         string
	BootstrapLogin    string
	BootstrapPassword string
	BootstrapName     string
	IdleSession       time.Duration
	AbsoluteSession   time.Duration
	SessionRenewEvery time.Duration
	QuoteTTL          time.Duration
}

func Load() (Config, error) {
	keyHex := getenv("SHOP_CREDENTIAL_KEY", "0123456789abcdef0123456789abcdef")
	key := []byte(keyHex)
	if len(key) != 32 {
		return Config{}, fmt.Errorf("SHOP_CREDENTIAL_KEY must be 32 bytes")
	}
	cfg := Config{
		Env:               getenv("SHOP_ENV", "development"),
		HTTPAddr:          getenv("SHOP_HTTP_ADDR", ":8080"),
		PublicBaseURL:     strings.TrimRight(getenv("SHOP_PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		MySQLDSN:          getenv("SHOP_MYSQL_DSN", "dishflow:dishflow@tcp(127.0.0.1:3306)/dishflow?parseTime=true&loc=UTC&charset=utf8mb4&multiStatements=true"),
		RedisAddr:         getenv("SHOP_REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword:     os.Getenv("SHOP_REDIS_PASSWORD"),
		RedisDB:           getenvInt("SHOP_REDIS_DB", 0),
		SessionSecret:     getenv("SHOP_SESSION_SECRET", "dev-session-secret-please-change-32b"),
		QuoteSecret:       getenv("SHOP_QUOTE_SECRET", "dev-quote-secret-please-change-32bytes"),
		CredentialKey:     key,
		PhoneHashPepper:   getenv("SHOP_PHONE_HASH_PEPPER", "dev-phone-pepper-change-me"),
		TrustedProxies:    splitCSV(os.Getenv("SHOP_TRUSTED_PROXIES")),
		DevMode:           getenvBool("SHOP_DEV_MODE", true),
		AdminDist:         getenv("SHOP_ADMIN_DIST", "apps/admin/dist"),
		UploadDir:         getenv("SHOP_UPLOAD_DIR", "data/uploads"),
		CookieSecure:      getenvBool("SHOP_COOKIE_SECURE", false),
		CookieDomain:      os.Getenv("SHOP_COOKIE_DOMAIN"),
		COSSecretID:       os.Getenv("COS_SECRET_ID"),
		COSSecretKey:      os.Getenv("COS_SECRET_KEY"),
		COSBucket:         os.Getenv("COS_BUCKET"),
		COSRegion:         os.Getenv("COS_REGION"),
		COSPrefix:         getenv("COS_PREFIX", "dishflow"),
		BootstrapLogin:    getenv("BOOTSTRAP_PLATFORM_LOGIN", "platform"),
		BootstrapPassword: getenv("BOOTSTRAP_PLATFORM_PASSWORD", "ChangeMeNow123"),
		BootstrapName:     getenv("BOOTSTRAP_PLATFORM_NAME", "平台管理员"),
		IdleSession:       8 * time.Hour,
		AbsoluteSession:   7 * 24 * time.Hour,
		SessionRenewEvery: 30 * time.Minute,
		QuoteTTL:          10 * time.Minute,
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Production() bool {
	return strings.EqualFold(c.Env, "production")
}

func (c Config) Validate() error {
	if c.Production() {
		if len(c.SessionSecret) < 32 {
			return fmt.Errorf("production SHOP_SESSION_SECRET is too short")
		}
		if len(c.QuoteSecret) < 32 {
			return fmt.Errorf("production SHOP_QUOTE_SECRET is too short")
		}
		if len(c.CredentialKey) != 32 {
			return fmt.Errorf("production SHOP_CREDENTIAL_KEY must be 32 bytes")
		}
		if strings.TrimSpace(c.PhoneHashPepper) == "" {
			return fmt.Errorf("production SHOP_PHONE_HASH_PEPPER must not be empty")
		}
	}
	if c.MySQLDSN == "" {
		return fmt.Errorf("SHOP_MYSQL_DSN is required")
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getenvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func splitCSV(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
