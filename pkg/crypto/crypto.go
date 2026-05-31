package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Encrypt encrypts plain text using AES-256-GCM with the provided key.
// The key should be 32 bytes (raw or base64 encoded).
func Encrypt(plainText string, key string) (string, error) {
	if len(plainText) == 0 {
		return "", nil
	}

	var cipherKey []byte
	decodedKey, err := base64.StdEncoding.DecodeString(key)
	if err == nil && len(decodedKey) == 32 {
		cipherKey = decodedKey
	} else {
		cipherKey = []byte(key)
		if len(cipherKey) < 32 {
			return "", errors.New("encryption key must be at least 32 bytes long")
		}
		cipherKey = cipherKey[:32]
	}

	block, err := aes.NewCipher(cipherKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts base64 encoded ciphertext encrypted with AES-256-GCM.
func Decrypt(cipherTextStr string, key string) (string, error) {
	if len(cipherTextStr) == 0 {
		return "", nil
	}

	var cipherKey []byte
	decodedKey, err := base64.StdEncoding.DecodeString(key)
	if err == nil && len(decodedKey) == 32 {
		cipherKey = decodedKey
	} else {
		cipherKey = []byte(key)
		if len(cipherKey) < 32 {
			return "", errors.New("encryption key must be at least 32 bytes long")
		}
		cipherKey = cipherKey[:32]
	}

	ciphertext, err := base64.StdEncoding.DecodeString(cipherTextStr)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(cipherKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertextActual := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextActual, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
