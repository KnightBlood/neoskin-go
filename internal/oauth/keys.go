package oauth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
)

// Keys 是 Passport 的 RSA 密钥对（storage/oauth-private.key / oauth-public.key）。
type Keys struct {
	Private *rsa.PrivateKey
	Public  *rsa.PublicKey
}

func LoadKeys(storageRoot string) (*Keys, error) {
	privatePath := filepath.Join(storageRoot, "oauth-private.key")
	publicPath := filepath.Join(storageRoot, "oauth-public.key")

	privateKey, err := loadPrivateKey(privatePath)
	if err != nil {
		return nil, err
	}
	publicKey, err := loadPublicKey(publicPath)
	if err != nil {
		return nil, err
	}

	return &Keys{Private: privateKey, Public: publicKey}, nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, errors.New("invalid oauth private key")
		}
		if key, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes); parseErr == nil {
			return key, nil
		}
		if key, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes); parseErr == nil {
			if rsaKey, ok := key.(*rsa.PrivateKey); ok {
				return rsaKey, nil
			}
		}
		return nil, errors.New("invalid oauth private key")
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	pemData := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	if err := os.WriteFile(path, pemData, 0o600); err != nil {
		return nil, err
	}
	publicPath := filepath.Join(filepath.Dir(path), "oauth-public.key")
	publicPEM := pem.EncodeToMemory(&pem.Block{
		Type: "PUBLIC KEY", Bytes: x509.MarshalPKCS1PublicKey(&key.PublicKey),
	})
	_ = os.WriteFile(publicPath, publicPEM, 0o644)
	return key, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid oauth public key")
	}
	if key, parseErr := x509.ParsePKCS1PublicKey(block.Bytes); parseErr == nil {
		return key, nil
	}
	if key, parseErr := x509.ParsePKIXPublicKey(block.Bytes); parseErr == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
	}
	return nil, errors.New("invalid oauth public key")
}
