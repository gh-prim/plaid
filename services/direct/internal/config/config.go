// Package config reads the service's settings from the environment.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/gh-prim/plaid/services/direct/internal/api"
)

// Config holds every setting.
type Config struct {
	Zone           string
	PublicIP       netip.Addr // required when DNS is on
	PublicIPv6     netip.Addr // optional
	HTTPAddr       string
	DNSAddr        string // "off" disables DNS
	DataDir        string
	TrustedProxies []netip.Prefix
	SiteURL        string
	AdminEmail     string
}

// DNSEnabled reports whether the DNS server should run.
func (c Config) DNSEnabled() bool { return c.DNSAddr != "off" }

// Load reads the configuration through getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}
	c := Config{
		Zone:       strings.TrimSuffix(strings.ToLower(get("DIRECT_ZONE", "getplaid.direct")), "."),
		HTTPAddr:   get("DIRECT_HTTP_ADDR", ":8080"),
		DNSAddr:    get("DIRECT_DNS_ADDR", ":53"),
		DataDir:    get("DIRECT_DATA_DIR", "./data"),
		SiteURL:    get("DIRECT_SITE_URL", "https://getplaid.tv"),
		AdminEmail: get("DIRECT_ADMIN_EMAIL", "hostmaster@"+get("DIRECT_ZONE", "getplaid.direct")),
	}

	var errs []error
	if v := get("DIRECT_PUBLIC_IP", ""); v != "" {
		ip, err := netip.ParseAddr(v)
		if err != nil || !ip.Is4() {
			errs = append(errs, fmt.Errorf("DIRECT_PUBLIC_IP: %q is not an IPv4 address", v))
		}
		c.PublicIP = ip
	} else if c.DNSEnabled() {
		errs = append(errs, errors.New("DIRECT_PUBLIC_IP is required for DNS (or set DIRECT_DNS_ADDR=off)"))
	}
	if v := get("DIRECT_PUBLIC_IPV6", ""); v != "" {
		ip, err := netip.ParseAddr(v)
		if err != nil || !ip.Is6() || ip.Is4In6() {
			errs = append(errs, fmt.Errorf("DIRECT_PUBLIC_IPV6: %q is not an IPv6 address", v))
		}
		c.PublicIPv6 = ip
	}

	c.TrustedProxies = api.DefaultTrustedProxies
	if v := get("DIRECT_TRUSTED_PROXIES", ""); v != "" {
		c.TrustedProxies = nil
		for _, s := range strings.Split(v, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			p, err := netip.ParsePrefix(s)
			if err != nil {
				// A bare address means a single host.
				a, aerr := netip.ParseAddr(s)
				if aerr != nil {
					errs = append(errs, fmt.Errorf("DIRECT_TRUSTED_PROXIES: %q is not a CIDR", s))
					continue
				}
				p = netip.PrefixFrom(a, a.BitLen())
			}
			c.TrustedProxies = append(c.TrustedProxies, p.Masked())
		}
	}
	return c, errors.Join(errs...)
}
