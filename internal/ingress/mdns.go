package ingress

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/grandcat/zeroconf"
)

// GetLocalIPv4s retrieves non-loopback, active IPv4 addresses on the host.
func GetLocalIPv4s() ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate network interfaces: %w", err)
	}

	var ips []string
	for _, iface := range ifaces {
		// Filter out interfaces that are down or loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			if ip4 := ip.To4(); ip4 != nil {
				ips = append(ips, ip4.String())
			}
		}
	}

	return ips, nil
}

// MDNSConfig contains configuration for the local mDNS discovery responder.
type MDNSConfig struct {
	Instance string
	Host     string
	Domain   string
	Service  string
	Port     int
	IPs      []string
	Logger   *slog.Logger
}

// DefaultMDNSConfig provides default configuration for advertising sovereign.local.
func DefaultMDNSConfig(logger *slog.Logger) MDNSConfig {
	if logger == nil {
		logger = slog.Default()
	}
	return MDNSConfig{
		Instance: "sovereign",
		Host:     "sovereign",
		Domain:   "local.",
		Service:  "_http._tcp",
		Port:     80,
		Logger:   logger,
	}
}

// MDNSService manages the lifecycle of the zero-configuration mDNS responder.
type MDNSService struct {
	cfg     MDNSConfig
	server  *zeroconf.Server
	ips     []string
	running bool
	logger  *slog.Logger
	mu      sync.RWMutex
}

// NewMDNSService initializes an mDNS responder.
func NewMDNSService(cfg MDNSConfig) *MDNSService {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Instance == "" {
		cfg.Instance = "sovereign"
	}
	if cfg.Host == "" {
		cfg.Host = "sovereign"
	}
	if cfg.Domain == "" {
		cfg.Domain = "local."
	}
	if cfg.Service == "" {
		cfg.Service = "_http._tcp"
	}
	if cfg.Port == 0 {
		cfg.Port = 80
	}

	return &MDNSService{
		cfg:    cfg,
		logger: cfg.Logger,
	}
}

// Start resolves local LAN IPs and registers the mDNS service responder.
// If multicast port binding fails, it logs a warning and returns the error
// allowing callers to gracefully degrade without crashing.
func (m *MDNSService) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	ips := m.cfg.IPs
	if len(ips) == 0 {
		detected, err := GetLocalIPv4s()
		if err != nil {
			m.logger.Warn("mDNS: failed to detect local LAN IP addresses", "error", err)
		} else {
			ips = detected
		}
	}

	if len(ips) == 0 {
		m.logger.Warn("mDNS: no active LAN IPv4 addresses detected, using fallback localhost")
		ips = []string{"127.0.0.1"}
	}

	server, err := zeroconf.RegisterProxy(
		m.cfg.Instance,
		m.cfg.Service,
		m.cfg.Domain,
		m.cfg.Port,
		m.cfg.Host,
		ips,
		[]string{"path=/"},
		nil,
	)
	if err != nil {
		m.logger.Warn("mDNS responder failed to bind or initialize, continuing without zero-config discovery",
			"error", err,
			"host", m.cfg.Host,
			"port", m.cfg.Port,
		)
		return err
	}

	m.server = server
	m.running = true
	m.ips = ips
	m.logger.Info("mDNS responder broadcasting",
		"hostname", m.cfg.Host+"."+strings.Trim(m.cfg.Domain, "."),
		"service", m.cfg.Service,
		"port", m.cfg.Port,
		"ips", ips,
	)
	return nil
}

// Close stops the mDNS responder and frees network resources.
func (m *MDNSService) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.server != nil {
		m.server.Shutdown()
		m.server = nil
	}
	m.running = false
	m.logger.Info("mDNS responder stopped")
}

// IsActive returns whether the mDNS responder is actively broadcasting.
func (m *MDNSService) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// IPs returns the IP addresses currently being advertised.
func (m *MDNSService) IPs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string(nil), m.ips...)
}

// Hostname returns the full advertised mDNS hostname (e.g. sovereign.local).
func (m *MDNSService) Hostname() string {
	return m.cfg.Host + "." + strings.Trim(m.cfg.Domain, ".")
}
