package oauth

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

// KeyManager manages signing and JWKS representation.
type KeyManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	kid        string
}

// LoadKeyManagerFromEnv loads an RSA private key from env or file.
func LoadKeyManagerFromEnv() (*KeyManager, error) {
	pemValue := os.Getenv("OAUTH_PRIVATE_KEY_PEM")
	if pemValue == "" {
		if path := os.Getenv("OAUTH_PRIVATE_KEY_PATH"); path != "" {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read OAUTH_PRIVATE_KEY_PATH: %w", err)
			}
			pemValue = string(data)
		}
	}
	if pemValue == "" {
		return nil, fmt.Errorf("OAUTH_PRIVATE_KEY_PEM or OAUTH_PRIVATE_KEY_PATH is required")
	}
	pemValue = strings.ReplaceAll(pemValue, `\n`, "\n")

	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("invalid private key PEM")
	}

	var key *rsa.PrivateKey
	if parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = parsed
	} else if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := parsed.(*rsa.PrivateKey); ok {
			key = rsaKey
		} else {
			return nil, fmt.Errorf("private key is not RSA")
		}
	} else {
		return nil, fmt.Errorf("unable to parse RSA private key")
	}

	pub := &key.PublicKey
	kid, err := computeKID(pub)
	if err != nil {
		return nil, err
	}

	return &KeyManager{
		privateKey: key,
		publicKey:  pub,
		kid:        kid,
	}, nil
}

func (k *KeyManager) PrivateKey() *rsa.PrivateKey {
	return k.privateKey
}

func (k *KeyManager) PublicKey() *rsa.PublicKey {
	return k.publicKey
}

func (k *KeyManager) KID() string {
	return k.kid
}

func computeKID(pub *rsa.PublicKey) (string, error) {
	derBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}
	sum := sha256.Sum256(derBytes)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}
