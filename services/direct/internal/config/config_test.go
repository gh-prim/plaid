package config

import (
	"testing"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoad(t *testing.T) {
	c, err := Load(env(map[string]string{"DIRECT_PUBLIC_IP": "192.0.2.53"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Zone != "getplaid.direct" || c.HTTPAddr != ":8080" || c.DNSAddr != ":53" || c.DataDir != "./data" ||
		c.SiteURL != "https://getplaid.tv" || c.AdminEmail != "hostmaster@getplaid.direct" || len(c.TrustedProxies) != 6 {
		t.Fatalf("defaults: %+v", c)
	}

	c, err = Load(env(map[string]string{
		"DIRECT_ZONE": "Example.Test.", "DIRECT_DNS_ADDR": "off",
		"DIRECT_TRUSTED_PROXIES": "10.1.0.0/16, 192.168.5.4", "DIRECT_PUBLIC_IPV6": "2001:db8::1",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Zone != "example.test" || c.DNSEnabled() || len(c.TrustedProxies) != 2 || c.TrustedProxies[1].String() != "192.168.5.4/32" {
		t.Fatalf("custom: %+v", c)
	}

	for _, bad := range []map[string]string{
		{}, // DNS on without a public IP
		{"DIRECT_PUBLIC_IP": "2001:db8::1"},
		{"DIRECT_PUBLIC_IP": "192.0.2.53", "DIRECT_PUBLIC_IPV6": "192.0.2.1"},
		{"DIRECT_PUBLIC_IP": "192.0.2.53", "DIRECT_TRUSTED_PROXIES": "nope"},
	} {
		if _, err := Load(env(bad)); err == nil {
			t.Errorf("%v: want an error", bad)
		}
	}
}
