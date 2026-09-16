// Package crypto encrypts secrets (SSH private keys, SAS tokens) at rest
// using AES-256-GCM directly with a master key from the environment.
//
// This deliberately skips a full envelope-encryption scheme (per-record DEK
// wrapped by a KEK): at this system's scale (a personal/internal ops tool
// managing a handful of servers) a wrapped-DEK-per-row buys no real security
// margin over direct AEAD — if the master key is compromised, both schemes
// fail identically. Confirmed with the user before implementation.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

var ErrDecryptFailed = errors.New("crypto: decryption failed (wrong key or corrupted/tampered ciphertext)")

// Sealer encrypts and decrypts secret columns with a master key, optionally
// falling back to a previous master key so secrets can be re-encrypted
// (rotated) without downtime.
type Sealer struct {
	current  cipher.AEAD
	previous cipher.AEAD // nil if no rotation in progress
}

func NewSealer(masterKey, previousMasterKey []byte) (*Sealer, error) {
	current, err := newAEAD(masterKey)
	if err != nil {
		return nil, fmt.Errorf("current master key: %w", err)
	}

	s := &Sealer{current: current}

	if len(previousMasterKey) > 0 {
		previous, err := newAEAD(previousMasterKey)
		if err != nil {
			return nil, fmt.Errorf("previous master key: %w", err)
		}
		s.previous = previous
	}

	return s, nil
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// associatedData binds ciphertext to the row/column it belongs to, so a
// ciphertext value can't be silently copied into a different row or column.
func associatedData(recordID, column string) []byte {
	return []byte(recordID + ":" + column)
}

// Encrypt seals plaintext as nonce||ciphertext||tag, always using the
// current master key.
func (s *Sealer) Encrypt(recordID, column, plaintext string) ([]byte, error) {
	nonce := make([]byte, s.current.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	ad := associatedData(recordID, column)
	return s.current.Seal(nonce, nonce, []byte(plaintext), ad), nil
}

// Decrypt opens a value sealed by Encrypt. It tries the current master key
// first, then the previous one (if configured) — this is what makes key
// rotation possible without a lockstep re-encryption of every row.
func (s *Sealer) Decrypt(recordID, column string, sealed []byte) (string, error) {
	ad := associatedData(recordID, column)

	if plaintext, err := open(s.current, sealed, ad); err == nil {
		return plaintext, nil
	}

	if s.previous != nil {
		if plaintext, err := open(s.previous, sealed, ad); err == nil {
			return plaintext, nil
		}
	}

	return "", ErrDecryptFailed
}

func open(aead cipher.AEAD, sealed, ad []byte) (string, error) {
	nonceSize := aead.NonceSize()
	if len(sealed) < nonceSize {
		return "", ErrDecryptFailed
	}
	nonce, ciphertext := sealed[:nonceSize], sealed[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, ad)
	if err != nil {
		return "", ErrDecryptFailed
	}
	return string(plaintext), nil
}
