package laravel

import (
	"os"
	"strings"
	"testing"
	"time"
)

const testKey = "base64:MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func TestKey(t *testing.T) {
	t.Setenv("NEOSKIN_APP_KEY", testKey)
	key, err := Key()
	if err != nil {
		t.Fatal(err)
	}
	if string(key) != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("key=%q", key)
	}
	raw, err := RawKey()
	if err != nil || raw != testKey {
		t.Fatalf("raw=%q err=%v", raw, err)
	}
}

func TestHS256(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	token, err := SignHS256(map[string]any{
		"sub": "1", "csrf": "abc", "exp": time.Now().Add(time.Hour).Unix(),
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	claims := VerifyHS256(token, key)
	if claims == nil || claims["sub"] != "1" || claims["csrf"] != "abc" {
		t.Fatalf("claims=%v", claims)
	}
	if VerifyHS256(token+"x", key) != nil {
		t.Fatal("tampered token must fail")
	}
}

func TestValidSignatureMatchesPHP(t *testing.T) {
	t.Setenv("NEOSKIN_APP_KEY", testKey)
	now := time.Unix(1767225000, 0)
	query := "expires=1767225600&signature=add412bc21360c107b45b5f2ca37be2493ff9b95caf9a11674c04bf5a284ae84"
	if !ValidSignature("/auth/reset/1", query, now) {
		t.Fatal("signature should be valid")
	}
	if ValidSignature("/auth/reset/2", query, now) {
		t.Fatal("different path must fail")
	}
	if ValidSignature("/auth/reset/1", query, time.Unix(1767225700, 0)) {
		t.Fatal("expired signature must fail")
	}
}

func TestKeyMissing(t *testing.T) {
	t.Setenv("NEOSKIN_APP_KEY", "")
	if _, err := Key(); err == nil {
		t.Fatal("expected error")
	}
	_ = os.Unsetenv("NEOSKIN_APP_KEY")
}

func TestSignedPathRoundTrip(t *testing.T) {
	t.Setenv("NEOSKIN_APP_KEY", testKey)
	expires := time.Unix(1767225600, 0)
	signed, err := SignedPath("/auth/reset/1", map[string]string{"uid": "1"}, &expires)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(signed, "/auth/reset/1?") {
		t.Fatalf("signed=%q", signed)
	}
	query := strings.TrimPrefix(signed, "/auth/reset/1?")
	if !ValidSignature("/auth/reset/1", query, time.Unix(1767225000, 0)) {
		t.Fatalf("signature invalid: %s", signed)
	}
	if ValidSignature("/auth/reset/2", query, time.Unix(1767225000, 0)) {
		t.Fatal("different path must fail")
	}
}
