package yggdrasil

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

// KeyPair — RSA-ключ Yggdrasil-сервера. Публичный ключ публикуется в /meta
// (поле signaturePublickey), приватным подписываются свойства текстур.
type KeyPair struct {
	private   *rsa.PrivateKey
	publicPEM string
}

// LoadOrCreateKey читает приватный ключ из path или генерирует новый и сохраняет.
func LoadOrCreateKey(path string) (*KeyPair, error) {
	if data, err := os.ReadFile(path); err == nil {
		if block, _ := pem.Decode(data); block != nil {
			if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
				if rsaKey, ok := key.(*rsa.PrivateKey); ok {
					return newKeyPair(rsaKey)
				}
			}
			if rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				return newKeyPair(rsaKey)
			}
		}
	}

	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("не удалось сгенерировать RSA-ключ: %w", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		_ = os.WriteFile(path, pemBytes, 0o600)
	}
	return newKeyPair(key)
}

func newKeyPair(key *rsa.PrivateKey) (*KeyPair, error) {
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	return &KeyPair{private: key, publicPEM: string(pubPEM)}, nil
}

// PublicKeyPEM возвращает публичный ключ в формате PEM (для поля signaturePublickey).
func (k *KeyPair) PublicKeyPEM() string {
	return k.publicPEM
}

// SignBase64 подписывает данные (SHA1withRSA) и возвращает base64 — формат
// подписи свойства текстур в Yggdrasil.
func (k *KeyPair) SignBase64(data []byte) (string, error) {
	digest := sha1.Sum(data)
	signature, err := rsa.SignPKCS1v15(rand.Reader, k.private, crypto.SHA1, digest[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}
