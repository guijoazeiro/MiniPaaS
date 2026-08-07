package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func TestRoundtrip(t *testing.T) {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	c, err := New(key)
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("postgres://user:secret@db/app")

	ct, nonce, err := c.Encrypt(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ct, plain) {
		t.Fatal("ciphertext equals plaintext")
	}
	got, err := c.Decrypt(ct, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("decrypt mismatch: got %q want %q", got, plain)
	}

	ct[0] ^= 0xff
	if _, err := c.Decrypt(ct, nonce); err == nil {
		t.Fatal("expected error on tampered ciphertext")
	}
}

func TestKeyLength(t *testing.T) {
	if _, err := New(make([]byte, 16)); err == nil {
		t.Fatal("expected error for 16-byte key")
	}
}
