package cryptoutil_test

import (
	"testing"

	"github.com/changerr/dishflow-grok/internal/cryptoutil"
)

func TestPasswordAndSecretRoundTrip(t *testing.T) {
	hash, err := cryptoutil.HashPassword("ChangeMeNow123")
	if err != nil {
		t.Fatal(err)
	}
	if !cryptoutil.VerifyPassword(hash, "ChangeMeNow123") {
		t.Fatal("verify")
	}
	if cryptoutil.VerifyPassword(hash, "wrong-password") {
		t.Fatal("should fail")
	}
	key := []byte("0123456789abcdef0123456789abcdef")
	enc, err := cryptoutil.Encrypt(key, []byte("secret-value"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := cryptoutil.Decrypt(key, enc)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "secret-value" {
		t.Fatalf("got %s", plain)
	}
}
