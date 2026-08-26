package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"os"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

// RegisterTLSConfig registers a custom TLS config named "aiven" using the
// Aiven CA certificate. The certificate can be supplied two ways:
//
//   - DB_CA_CERT_PATH: path to a mounted cert file (used on platforms like
//     Render that support Secret Files).
//   - DB_CA_CERT_BASE64: the cert content base64-encoded directly as an
//     env var (used on platforms without file-mounting, e.g. SnapDeploy).
//
// If neither is set (local dev without TLS), it does nothing and the plain
// DSN is used instead.
func RegisterTLSConfig() error {
	caCert, err := loadCACert()
	if err != nil {
		return err
	}
	if caCert == nil {
		return nil
	}

	rootCertPool := x509.NewCertPool()
	if ok := rootCertPool.AppendCertsFromPEM(caCert); !ok {
		return fmt.Errorf("failed to append CA cert to pool")
	}

	return mysqlDriver.RegisterTLSConfig("aiven", &tls.Config{
		RootCAs: rootCertPool,
	})
}

// loadCACert returns the CA certificate bytes from whichever source is
// configured, or nil if neither is set.
func loadCACert() ([]byte, error) {
	if b64 := os.Getenv("DB_CA_CERT_BASE64"); b64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode DB_CA_CERT_BASE64: %w", err)
		}
		return decoded, nil
	}

	if path := os.Getenv("DB_CA_CERT_PATH"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert at %s: %w", path, err)
		}
		return data, nil
	}

	return nil, nil
}
