package support

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Certificate struct {
	Version         string            `json:"version"`
	GeneratedAtUtc  string            `json:"generatedAtUtc"`
	Mode            string            `json:"mode"`
	Pass            bool              `json:"pass"`
	Reason          string            `json:"reason"`
	RedCount        int               `json:"redCount"`
	AmberCount      int               `json:"amberCount"`
	GreenCount      int               `json:"greenCount"`
	MaxRed          *int              `json:"max_red,omitempty"`
	MaxAmber        *int              `json:"max_amber,omitempty"`
	Policy          PolicyInfo        `json:"policy"`
	EvidenceHashes  map[string]string `json:"evidence_hashes,omitempty"`
	Signature       string            `json:"signature,omitempty"`
	SignatureMethod string            `json:"signature_method,omitempty"`
}

type PolicyInfo struct {
	FailOnRed  bool `json:"fail_on_red"`
	AllowAmber bool `json:"allow_amber"`
}

func NewCertificate(mode string) Certificate {
	return Certificate{
		Version:        "1.0",
		GeneratedAtUtc: time.Now().UTC().Format(time.RFC3339),
		Mode:           mode,
		Policy: PolicyInfo{
			FailOnRed:  true,
			AllowAmber: false,
		},
	}
}

func SignCertificate(cert *Certificate, priv ed25519.PrivateKey) error {
	payload, err := marshalCertPayload(cert)
	if err != nil {
		return err
	}
	sig := ed25519.Sign(priv, payload)
	cert.Signature = base64.StdEncoding.EncodeToString(sig)
	cert.SignatureMethod = "ed25519"
	return nil
}

func VerifyCertificate(cert *Certificate, pub ed25519.PublicKey) (bool, error) {
	if cert.Signature == "" {
		return false, errors.New("missing signature")
	}
	sig, err := base64.StdEncoding.DecodeString(cert.Signature)
	if err != nil {
		return false, err
	}
	payload, err := marshalCertPayload(cert)
	if err != nil {
		return false, err
	}
	return ed25519.Verify(pub, payload, sig), nil
}

func LoadSigningKey(workspace string) (ed25519.PrivateKey, error) {
	if env := os.Getenv("GRES_SIGNING_PRIVATE_KEY"); env != "" {
		return decodePrivateKey(env)
	}
	path := filepath.Join(workspace, ".b2b", "keys", "signing_ed25519")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodePrivateKey(string(data))
}

func decodePrivateKey(raw string) (ed25519.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty key")
	}
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return normalizePrivateKey(b)
	}
	if b, err := hex.DecodeString(raw); err == nil {
		return normalizePrivateKey(b)
	}
	return nil, errors.New("invalid private key format")
}

func normalizePrivateKey(b []byte) (ed25519.PrivateKey, error) {
	if len(b) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(b), nil
	}
	if len(b) == ed25519.PrivateKeySize {
		return ed25519.PrivateKey(b), nil
	}
	return nil, fmt.Errorf("invalid key length: %d", len(b))
}

func marshalCertPayload(cert *Certificate) ([]byte, error) {
	tmp := *cert
	tmp.Signature = ""
	tmp.SignatureMethod = ""
	return json.Marshal(tmp)
}

func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
