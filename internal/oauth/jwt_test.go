package oauth

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
)

func TestJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys := &Keys{Private: key, Public: &key.PublicKey}
	claims := map[string]any{"sub": "1", "scopes": []string{"User.Read"}}
	token, err := SignRS256(claims, keys)
	if err != nil {
		t.Fatal(err)
	}
	decoded := VerifyRS256(token, keys)
	if decoded == nil || decoded["sub"] != "1" {
		t.Fatalf("decoded=%v", decoded)
	}

	if VerifyRS256(token+"x", keys) != nil {
		t.Fatal("tampered token must fail")
	}
}
