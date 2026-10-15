package ingress

import (
	"net"
	"testing"
)

func TestMDNS_GetLocalIPv4s(t *testing.T) {
	ips, err := GetLocalIPv4s()
	if err != nil {
		t.Fatalf("unexpected error getting local IPs: %v", err)
	}

	for _, ipStr := range ips {
		parsed := net.ParseIP(ipStr)
		if parsed == nil {
			t.Errorf("expected valid IP string, got %s", ipStr)
		}
		if parsed.To4() == nil {
			t.Errorf("expected IPv4 address, got %s", ipStr)
		}
		if parsed.IsLoopback() {
			t.Errorf("expected non-loopback IP, got %s", ipStr)
		}
	}
}

func TestMDNS_ServiceConfiguration(t *testing.T) {
	cfg := DefaultMDNSConfig(nil)
	cfg.IPs = []string{"192.168.1.50"}

	svc := NewMDNSService(cfg)
	if svc.Hostname() != "sovereign.local" {
		t.Errorf("expected hostname sovereign.local, got %s", svc.Hostname())
	}
	if svc.IsActive() {
		t.Errorf("expected service to be inactive before Start()")
	}
}

func TestMDNS_StartAndGracefulDegradation(t *testing.T) {
	cfg := DefaultMDNSConfig(nil)
	cfg.IPs = []string{"127.0.0.1"}

	svc := NewMDNSService(cfg)

	// Attempt start: whether port 5353 is available or in use by host avahi/resolved,
	// Start() must NOT panic and must handle errors gracefully.
	err := svc.Start()
	if err != nil {
		// Port conflict or interface restriction: verify graceful degradation
		if svc.IsActive() {
			t.Errorf("service should not be marked active when Start() fails")
		}
		// Calling Close on failed service must be a safe no-op
		svc.Close()
	} else {
		// Successfully bound: verify active status and clean shutdown
		if !svc.IsActive() {
			t.Errorf("expected service to be active after successful Start()")
		}
		if len(svc.IPs()) == 0 {
			t.Errorf("expected non-empty advertised IPs")
		}
		svc.Close()
		if svc.IsActive() {
			t.Errorf("expected service to be inactive after Close()")
		}
	}
}
