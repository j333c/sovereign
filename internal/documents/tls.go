package documents

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// BuildTLSConfig constructs a *tls.Config configured with the server hostname,
// optional InsecureSkipVerify, and an optional custom CA root certificate pool.
func BuildTLSConfig(host string, insecureSkipVerify bool, caCertPath string) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: insecureSkipVerify,
	}

	if caCertPath != "" {
		certBytes, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate at %q: %w", caCertPath, err)
		}

		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if ok := pool.AppendCertsFromPEM(certBytes); !ok {
			return nil, fmt.Errorf("failed to parse CA certificate from PEM at %q", caCertPath)
		}
		tlsConfig.RootCAs = pool
	}

	return tlsConfig, nil
}
