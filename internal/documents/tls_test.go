package documents

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateSelfSignedCert creates a self-signed CA / server certificate for testing.
func generateSelfSignedCert(t *testing.T, host string) (certPEM []byte, keyPEM []byte, tlsCert tls.Certificate) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	notBefore := time.Now().Add(-1 * time.Hour)
	notAfter := time.Now().Add(24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		t.Fatalf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Sovereign Test CA"},
			CommonName:   host,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes})

	tlsCert, err = tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("failed to create X509KeyPair: %v", err)
	}

	return certPEM, keyPEM, tlsCert
}

func TestIMAP_CustomTLS(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("InsecureSkipVerify flag configures TLS properly", func(t *testing.T) {
		cfg, err := BuildTLSConfig("imap.example.local", true, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.InsecureSkipVerify {
			t.Errorf("expected InsecureSkipVerify to be true")
		}
		if cfg.ServerName != "imap.example.local" {
			t.Errorf("expected ServerName to be imap.example.local, got %s", cfg.ServerName)
		}
		if cfg.RootCAs != nil {
			t.Errorf("expected RootCAs to be nil when no CA cert path provided")
		}
	})

	t.Run("Custom CA certificate loaded into RootCAs", func(t *testing.T) {
		certPEM, _, _ := generateSelfSignedCert(t, "mail.home.arpa")
		caPath := filepath.Join(tmpDir, "custom-ca.crt")
		if err := os.WriteFile(caPath, certPEM, 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := BuildTLSConfig("mail.home.arpa", false, caPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.InsecureSkipVerify {
			t.Errorf("expected InsecureSkipVerify to be false")
		}
		if cfg.ServerName != "mail.home.arpa" {
			t.Errorf("expected ServerName to be mail.home.arpa, got %s", cfg.ServerName)
		}
		if cfg.RootCAs == nil {
			t.Fatalf("expected RootCAs cert pool to be populated")
		}
	})

	t.Run("Non-existent CA file returns descriptive error", func(t *testing.T) {
		_, err := BuildTLSConfig("mail.home.arpa", false, filepath.Join(tmpDir, "non-existent.crt"))
		if err == nil {
			t.Fatalf("expected error for non-existent CA file, got nil")
		}
	})

	t.Run("Malformed CA file returns error", func(t *testing.T) {
		badPath := filepath.Join(tmpDir, "bad-ca.crt")
		if err := os.WriteFile(badPath, []byte("NOT A VALID PEM CERTIFICATE"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := BuildTLSConfig("mail.home.arpa", false, badPath)
		if err == nil {
			t.Fatalf("expected error for malformed PEM CA file, got nil")
		}
	})

	t.Run("End-to-End TLS connection verification with custom CA and InsecureSkipVerify", func(t *testing.T) {
		host := "127.0.0.1"
		certPEM, _, tlsCert := generateSelfSignedCert(t, host)

		caPath := filepath.Join(tmpDir, "server-ca.crt")
		if err := os.WriteFile(caPath, certPEM, 0644); err != nil {
			t.Fatal(err)
		}

		serverTLSConfig := &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		}

		listener, err := tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig)
		if err != nil {
			t.Fatalf("failed to create TLS listener: %v", err)
		}
		defer listener.Close()

		// Server accept loop responding with mock greeting
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				go func(c net.Conn) {
					defer c.Close()
					_, _ = c.Write([]byte("* OK IMAP4rev1 Test Server Ready\r\n"))
					buf := make([]byte, 1024)
					_, _ = c.Read(buf)
				}(conn)
			}
		}()

		serverAddr := listener.Addr().String()

		// Case A: Default verify without custom CA -> FAILS (unknown authority)
		strictConfig, err := BuildTLSConfig(host, false, "")
		if err != nil {
			t.Fatal(err)
		}
		connA, errA := tls.Dial("tcp", serverAddr, strictConfig)
		if errA == nil {
			_ = connA.Close()
			t.Fatalf("expected self-signed certificate to fail verification without custom CA")
		}

		// Case B: With InsecureSkipVerify = true -> SUCCEEDS
		insecureConfig, err := BuildTLSConfig(host, true, "")
		if err != nil {
			t.Fatal(err)
		}
		connB, errB := tls.Dial("tcp", serverAddr, insecureConfig)
		if errB != nil {
			t.Fatalf("expected InsecureSkipVerify=true to succeed, got: %v", errB)
		}
		_ = connB.Close()

		// Case C: With valid custom CA cert path -> SUCCEEDS
		customCAConfig, err := BuildTLSConfig(host, false, caPath)
		if err != nil {
			t.Fatal(err)
		}
		connC, errC := tls.Dial("tcp", serverAddr, customCAConfig)
		if errC != nil {
			t.Fatalf("expected connection with custom CA pool to succeed, got: %v", errC)
		}
		_ = connC.Close()
	})
}
