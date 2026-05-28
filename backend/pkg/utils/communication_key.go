package utils

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

type CommunicationKey struct {
	privateKey *rsa.PrivateKey
}

// NewCommunicationKey generates or loads RSA-2048 PKCS#1 key pair from storageDir/comkey/
// Files: ipc-priv.pem (0600), ipc-pub.pem (0644)
func NewCommunicationKey(storageDir string) (*CommunicationKey, error) {
	keyDir := filepath.Join(storageDir, "comkey")
	if err := os.MkdirAll(keyDir, 0755); err != nil {
		return nil, fmt.Errorf("create comkey dir: %w", err)
	}

	privPath := filepath.Join(keyDir, "ipc-priv.pem")
	pubPath := filepath.Join(keyDir, "ipc-pub.pem")

	var privKey *rsa.PrivateKey

	if _, err := os.Stat(privPath); os.IsNotExist(err) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate rsa key: %w", err)
		}
		privKey = key

		privPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		})
		if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
			return nil, fmt.Errorf("write private key: %w", err)
		}

		pubPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey),
		})
		if err := os.WriteFile(pubPath, pubPEM, 0644); err != nil {
			return nil, fmt.Errorf("write public key: %w", err)
		}
	} else {
		privPEM, err := os.ReadFile(privPath)
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}
		block, _ := pem.Decode(privPEM)
		if block == nil {
			return nil, fmt.Errorf("failed to decode private key PEM")
		}
		privKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
	}

	return &CommunicationKey{privateKey: privKey}, nil
}

// Sign returns RSA-SHA256 signature of data as hex string.
func (c *CommunicationKey) Sign(data string) (string, error) {
	hash := sha256.Sum256([]byte(data))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return hex.EncodeToString(sig), nil
}

// PrivateSign returns RSA private-key signed data as base64 string.
// This provides authenticity (not confidentiality); the public key can verify it.
// In Go: rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.Hash(0), []byte(data))
func (c *CommunicationKey) PrivateSign(data string) (string, error) {
	signed, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.Hash(0), []byte(data))
	if err != nil {
		return "", fmt.Errorf("private sign: %w", err)
	}
	return base64.StdEncoding.EncodeToString(signed), nil
}
