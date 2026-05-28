package utils

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/scrypt"
)

type EncryptionManager struct {
	aesKey []byte
}

func NewEncryptionManager(storageDir string) (*EncryptionManager, error) {
	keyDir := filepath.Join(storageDir, "encryption")
	if err := os.MkdirAll(keyDir, 0755); err != nil {
		return nil, fmt.Errorf("create key dir: %w", err)
	}

	privPath := filepath.Join(keyDir, "private.pem")
	var privPEM []byte

	if _, err := os.Stat(privPath); os.IsNotExist(err) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate rsa key: %w", err)
		}
		privPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})
		if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
			return nil, fmt.Errorf("write private key: %w", err)
		}
	} else {
		var err error
		privPEM, err = os.ReadFile(privPath)
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}
	}

	hash := sha256.Sum256(privPEM)
	return &EncryptionManager{aesKey: hash[:]}, nil
}

func (e *EncryptionManager) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.aesKey)
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
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *EncryptionManager) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(e.aesKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, cipherText := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// deriveCBCKey reads SIG_KEY/SIG_SALT env (auto-generate 32-byte hex if missing),
// derives 32-byte key via scrypt.Key(key, salt, 32768, 8, 1, 32)
func (e *EncryptionManager) deriveCBCKey() []byte {
	keyHex := os.Getenv("SIG_KEY")
	if keyHex == "" {
		keyHex = hex.EncodeToString(e.aesKey)
	}
	saltHex := os.Getenv("SIG_SALT")
	if saltHex == "" {
		saltHex = hex.EncodeToString(e.aesKey)
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		key = e.aesKey
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		salt = e.aesKey
	}

	derived, err := scrypt.Key(key, salt, 32768, 8, 1, 32)
	if err != nil {
		return e.aesKey
	}
	return derived
}

// XPayload returns base64-encoded AES-CBC key for Collector header
func (e *EncryptionManager) XPayload() string {
	return base64.StdEncoding.EncodeToString(e.deriveCBCKey())
}

// EncryptCBC uses AES-256-CBC with PKCS#7 padding, format: hex(ciphertext):hex(iv)
func (e *EncryptionManager) EncryptCBC(plaintext string) (string, error) {
	key := e.deriveCBCKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	data := []byte(plaintext)
	padLen := aes.BlockSize - len(data)%aes.BlockSize
	padding := bytes.Repeat([]byte{byte(padLen)}, padLen)
	data = append(data, padding...)

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(data, data)

	return hex.EncodeToString(data) + ":" + hex.EncodeToString(iv), nil
}

// DecryptCBC reverses EncryptCBC
func (e *EncryptionManager) DecryptCBC(ciphertext string) (string, error) {
	parts := strings.Split(ciphertext, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid ciphertext format")
	}

	encrypted, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	iv, err := hex.DecodeString(parts[1])
	if err != nil {
		return "", err
	}

	key := e.deriveCBCKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(encrypted)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(encrypted, encrypted)

	padLen := int(encrypted[len(encrypted)-1])
	if padLen > aes.BlockSize || padLen == 0 {
		return "", fmt.Errorf("invalid padding")
	}
	for i := 0; i < padLen; i++ {
		if encrypted[len(encrypted)-1-i] != byte(padLen) {
			return "", fmt.Errorf("invalid padding")
		}
	}

	return string(encrypted[:len(encrypted)-padLen]), nil
}
