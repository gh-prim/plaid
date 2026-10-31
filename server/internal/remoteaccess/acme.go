package remoteaccess

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme"
)

// LetsEncryptURL is the production ACME directory.
const LetsEncryptURL = "https://acme-v02.api.letsencrypt.org/directory"

var errACME = errors.New("acme_failed")

// Issuer gets a certificate for host and *.host, faked in tests.
type Issuer interface {
	Issue(ctx context.Context, host string, publish func(ctx context.Context, values []string) error, unpublish func(ctx context.Context) error) (*tls.Certificate, error)
}

// ACMEIssuer gets certificates over ACME with DNS-01: getplaid.direct
// publishes the challenge, so the certificate's private key never leaves
// this machine and the server needs no port 80.
type ACMEIssuer struct {
	DirectoryURL string
	// KeyPath holds the account key, created on first use.
	KeyPath string
	// Propagation is the wait between publishing the TXT values and
	// asking the CA to check them. getplaid.direct is authoritative and
	// answers at once; this covers its own replicas.
	Propagation time.Duration
}

// Issue runs one order to completion.
func (a *ACMEIssuer) Issue(ctx context.Context, host string, publish func(context.Context, []string) error, unpublish func(context.Context) error) (*tls.Certificate, error) {
	key, err := loadOrCreateKey(a.KeyPath)
	if err != nil {
		return nil, err
	}
	client := &acme.Client{Key: key, DirectoryURL: a.DirectoryURL}
	if _, err := client.Register(ctx, &acme.Account{}, acme.AcceptTOS); err != nil && !errors.Is(err, acme.ErrAccountAlreadyExists) {
		return nil, fmt.Errorf("%w: register account: %w", errACME, err)
	}
	// The wildcard covers the a-b-c-d.<host> names that resolve to LAN
	// addresses, for clients at home.
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(host, "*."+host))
	if err != nil {
		return nil, fmt.Errorf("%w: order: %w", errACME, err)
	}
	var values []string
	var challenges []*acme.Challenge
	for _, u := range order.AuthzURLs {
		z, err := client.GetAuthorization(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("%w: authorization: %w", errACME, err)
		}
		if z.Status == acme.StatusValid {
			continue
		}
		for _, c := range z.Challenges {
			if c.Type != "dns-01" {
				continue
			}
			v, err := client.DNS01ChallengeRecord(c.Token)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errACME, err)
			}
			values, challenges = append(values, v), append(challenges, c)
		}
	}
	if len(values) > 0 {
		if err := publish(ctx, values); err != nil {
			return nil, err
		}
		defer func() { _ = unpublish(context.WithoutCancel(ctx)) }()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(a.Propagation):
		}
		for _, c := range challenges {
			if _, err := client.Accept(ctx, c); err != nil {
				return nil, fmt.Errorf("%w: accept: %w", errACME, err)
			}
		}
		for _, u := range order.AuthzURLs {
			if _, err := client.WaitAuthorization(ctx, u); err != nil {
				return nil, fmt.Errorf("%w: authorization: %w", errACME, err)
			}
		}
	}
	if order, err = client.WaitOrder(ctx, order.URI); err != nil {
		return nil, fmt.Errorf("%w: order: %w", errACME, err)
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{DNSNames: []string{host, "*." + host}}, certKey)
	if err != nil {
		return nil, err
	}
	der, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return nil, fmt.Errorf("%w: finalize: %w", errACME, err)
	}
	return certificate(der, certKey)
}

func certificate(der [][]byte, key crypto.Signer) (*tls.Certificate, error) {
	leaf, err := x509.ParseCertificate(der[0])
	if err != nil {
		return nil, err
	}
	return &tls.Certificate{Certificate: der, PrivateKey: key, Leaf: leaf}, nil
}

// saveCertificate writes the chain and key as PEM, the key readable by
// the server's user only.
func saveCertificate(dir string, c *tls.Certificate) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	var chain []byte
	for _, der := range c.Certificate {
		chain = append(chain, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(c.PrivateKey)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "key.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})); err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "cert.pem"), chain)
}

// loadCertificate reads what saveCertificate wrote; nil when absent.
func loadCertificate(dir string) (*tls.Certificate, error) {
	c, err := tls.LoadX509KeyPair(filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil // Leaf is filled in by LoadX509KeyPair since Go 1.23
}

func loadOrCreateKey(path string) (crypto.Signer, error) {
	if b, err := os.ReadFile(path); err == nil { //nolint:gosec // path is under the data dir, not user input
		block, _ := pem.Decode(b)
		if block == nil {
			return nil, fmt.Errorf("%s: not PEM", path)
		}
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		s, ok := k.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("%s: not a signing key", path)
		}
		return s, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return k, writeFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// writeFile replaces path atomically, so a crash never leaves half a key.
func writeFile(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
