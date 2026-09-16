package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func randomKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate random key: %v", err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	sealer, err := NewSealer(randomKey(t), nil)
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}

	sealed, err := sealer.Encrypt("record-1", "secret_column", "top secret value")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	plaintext, err := sealer.Decrypt("record-1", "secret_column", sealed)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if plaintext != "top secret value" {
		t.Fatalf("got %q, want %q", plaintext, "top secret value")
	}
}

func TestDecryptFailsWithWrongRecordID(t *testing.T) {
	sealer, err := NewSealer(randomKey(t), nil)
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}

	sealed, err := sealer.Encrypt("record-1", "secret_column", "value")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Simulates a ciphertext copied into a different row: the associated
	// data binding must make this fail rather than silently decrypt.
	if _, err := sealer.Decrypt("record-2", "secret_column", sealed); err == nil {
		t.Fatal("expected decryption to fail when record ID does not match, got nil error")
	}
}

func TestDecryptFailsWithWrongColumn(t *testing.T) {
	sealer, err := NewSealer(randomKey(t), nil)
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}

	sealed, err := sealer.Encrypt("record-1", "column_a", "value")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if _, err := sealer.Decrypt("record-1", "column_b", sealed); err == nil {
		t.Fatal("expected decryption to fail when column does not match, got nil error")
	}
}

func TestKeyRotationFallsBackToPreviousKey(t *testing.T) {
	oldKey := randomKey(t)
	newKey := randomKey(t)

	oldSealer, err := NewSealer(oldKey, nil)
	if err != nil {
		t.Fatalf("NewSealer(old): %v", err)
	}
	sealed, err := oldSealer.Encrypt("record-1", "col", "value encrypted with old key")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Rotated sealer: current = newKey, previous = oldKey. It should still
	// be able to decrypt data sealed under the old key.
	rotatedSealer, err := NewSealer(newKey, oldKey)
	if err != nil {
		t.Fatalf("NewSealer(rotated): %v", err)
	}

	plaintext, err := rotatedSealer.Decrypt("record-1", "col", sealed)
	if err != nil {
		t.Fatalf("Decrypt with rotated sealer: %v", err)
	}
	if plaintext != "value encrypted with old key" {
		t.Fatalf("got %q", plaintext)
	}

	// A sealer with only the new key (rotation fully complete, previous
	// key retired) must no longer be able to open old-key ciphertext.
	newOnlySealer, err := NewSealer(newKey, nil)
	if err != nil {
		t.Fatalf("NewSealer(new only): %v", err)
	}
	if _, err := newOnlySealer.Decrypt("record-1", "col", sealed); err == nil {
		t.Fatal("expected decryption to fail once the previous key is retired")
	}
}

func TestEncryptProducesDistinctCiphertextEachTime(t *testing.T) {
	sealer, err := NewSealer(randomKey(t), nil)
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}

	a, err := sealer.Encrypt("record-1", "col", "same plaintext")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	b, err := sealer.Encrypt("record-1", "col", "same plaintext")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if bytes.Equal(a, b) {
		t.Fatal("expected distinct ciphertexts due to random nonces, got identical output")
	}
}
