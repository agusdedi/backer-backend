package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

// RegisterTLSConfig registers a custom TLS config named "aiven" using the
// CA certificate at DB_CA_CERT_PATH. If the env var is empty (local dev
// without TLS), it does nothing and the plain DSN is used instead.
func RegisterTLSConfig() error {
	caCertPath := os.Getenv("DB_CA_CERT_PATH")
	if caCertPath == "" {
		return nil
	}

	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA cert at %s: %w", caCertPath, err)
	}

	rootCertPool := x509.NewCertPool()
	if ok := rootCertPool.AppendCertsFromPEM(caCert); !ok {
		return fmt.Errorf("failed to append CA cert to pool")
	}

	return mysqlDriver.RegisterTLSConfig("aiven", &tls.Config{
		RootCAs: rootCertPool,
	})
}
