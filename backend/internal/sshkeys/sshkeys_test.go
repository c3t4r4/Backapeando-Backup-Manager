package sshkeys

import (
	"golang.org/x/crypto/ssh"
	"strings"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	priv, pub, fp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	// Verify private and public keys are not empty
	if len(priv) == 0 || len(pub) == 0 {
		t.Fatal("private or public key is empty")
	}

	// Verify fingerprint format
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Errorf("fingerprint format: got %q, want SHA256:...", fp)
	}

	// Verify public key is valid authorized_keys format
	authedKey, _, _, _, err := ssh.ParseAuthorizedKey(pub)
	if err != nil {
		t.Fatalf("ssh.ParseAuthorizedKey failed: %v", err)
	}

	// Verify private key can be parsed
	privKey, err := ssh.ParsePrivateKey(priv)
	if err != nil {
		t.Fatalf("ssh.ParsePrivateKey failed: %v", err)
	}

	// Verify public key from private key matches generated public key
	privPubKey := privKey.PublicKey()
	if !pubKeysEqual(privPubKey, authedKey) {
		t.Error("private and public keys do not match")
	}
}

func TestFingerprintFormat(t *testing.T) {
	_, pub, fp, _ := GenerateKeyPair()
	authedKey, _, _, _, _ := ssh.ParseAuthorizedKey(pub)
	expectedFP := formatFingerprint(authedKey)
	if fp != expectedFP {
		t.Errorf("fingerprint mismatch: got %q, expected %q", fp, expectedFP)
	}
}

func TestDistinctKeys(t *testing.T) {
	_, pub1, fp1, _ := GenerateKeyPair()
	_, pub2, fp2, _ := GenerateKeyPair()
	if string(pub1) == string(pub2) || fp1 == fp2 {
		t.Error("two generated keys are identical (should be unique)")
	}
}

func pubKeysEqual(a, b ssh.PublicKey) bool {
	return string(a.Marshal()) == string(b.Marshal())
}
