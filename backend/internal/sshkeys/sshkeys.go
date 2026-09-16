// Package sshkeys handles SSH ed25519 key pair generation and fingerprint computation.
package sshkeys

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"golang.org/x/crypto/ssh"
)

// GenerateKeyPair generates a new ed25519 key pair and returns the private key in OpenSSH format,
// the public key in OpenSSH authorized_keys format, and the SHA256 fingerprint.
func GenerateKeyPair() (privateKeyPEM []byte, publicKeyOpenSSH []byte, fingerprint string, err error) {
	// Generate new ed25519 key pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, "", fmt.Errorf("ed25519.GenerateKey: %w", err)
	}

	// Private key in OpenSSH format
	privateKeyPEM, err = encodePrivateKeyOpenSSH(priv)
	if err != nil {
		return nil, nil, "", fmt.Errorf("encodePrivateKeyOpenSSH: %w", err)
	}

	// Public key in OpenSSH authorized_keys format
	pubSigner, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, nil, "", fmt.Errorf("ssh.NewPublicKey: %w", err)
	}
	publicKeyOpenSSH = ssh.MarshalAuthorizedKey(pubSigner)

	// SHA256 fingerprint (OpenSSH format)
	fingerprint = formatFingerprint(pubSigner)

	return privateKeyPEM, publicKeyOpenSSH, fingerprint, nil
}

// encodePrivateKeyOpenSSH encodes an ed25519 private key as OpenSSH format.
// This uses ssh.MarshalPrivateKey to serialize the key.
func encodePrivateKeyOpenSSH(priv ed25519.PrivateKey) ([]byte, error) {
	// MarshalPrivateKey expects (crypto.PrivateKey, comment string) and returns (*pem.Block, error)
	pemBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return nil, fmt.Errorf("MarshalPrivateKey: %w", err)
	}

	// Encode the PEM block to bytes
	return pem.EncodeToMemory(pemBlock), nil
}

// formatFingerprint computes the SHA256 fingerprint of an SSH public key in OpenSSH format.
func formatFingerprint(pubKey ssh.PublicKey) string {
	hash := sha256.Sum256(pubKey.Marshal())
	return "SHA256:" + base64.StdEncoding.EncodeToString(hash[:])
}
