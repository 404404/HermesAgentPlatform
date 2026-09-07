package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestParseOperationalSecretMasterKey(t *testing.T) {
	key := "01234567890123456789012345678901"
	parsed, err := parseOperationalSecretMasterKey(key)
	if err != nil || !bytes.Equal(parsed, []byte(key)) {
		t.Fatalf("raw master key parse failed: %v", err)
	}
	encoded := "base64:" + base64.StdEncoding.EncodeToString([]byte(key))
	parsed, err = parseOperationalSecretMasterKey(encoded)
	if err != nil || !bytes.Equal(parsed, []byte(key)) {
		t.Fatalf("base64 master key parse failed: %v", err)
	}
	if _, err = parseOperationalSecretMasterKey("short"); err == nil {
		t.Fatal("short master key must be rejected")
	}
}

func TestOperationalSecretEncryptionRoundTrip(t *testing.T) {
	p := &databaseSecretProvider{key: []byte("01234567890123456789012345678901")}
	ciphertext, nonce, err := p.encrypt([]byte("credential-value"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ciphertext, []byte("credential-value")) || len(nonce) == 0 {
		t.Fatal("secret must be encrypted with a nonce")
	}
	plaintext, err := p.decrypt(ciphertext, nonce)
	if err != nil || !bytes.Equal(plaintext, []byte("credential-value")) {
		t.Fatalf("secret must decrypt only through the authorized provider: %v", err)
	}
}
