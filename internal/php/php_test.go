package php

import "testing"

func TestVerifyPHPBcrypt(t *testing.T) {
	const hash = "$2y$12$et7QWmQ/rUtcXmSygxSRLOvFW9wdPNwZqX0Au8if0NCHdbtq3sUPC"

	if !VerifyPassword(hash, "test") {
		t.Fatal("expected $2y$ hash to verify")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("wrong password must not verify")
	}
}

func TestHashPasswordUsesPhpPrefix(t *testing.T) {
	hash, err := HashPassword("secret123", 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) != 60 || hash[0:4] != "$2y$" {
		t.Fatalf("unexpected hash: %q", hash)
	}
	if !VerifyPassword(hash, "secret123") {
		t.Fatal("generated hash must verify")
	}
}

func TestSha256Hex(t *testing.T) {
	if got := Sha256Hex([]byte("abc")); got !=
		"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("sha256=%s", got)
	}
}
