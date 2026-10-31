package dnsserver

import (
	"context"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/miekg/dns"

	"github.com/gh-prim/plaid/services/direct/internal/store"
)

type fixture struct {
	t    *testing.T
	udp  string
	tcp  string
	st   *store.Store
	now  time.Time
	srv  *store.Server
	dual *store.Server
}

func setup(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	now := time.Unix(1_800_000_000, 0).UTC()

	v4only := &store.Server{Slug: "living-room", TokenHash: []byte{1}, Port: 32501, CreatedAt: now}
	v4only.Seen(netip.MustParseAddr("203.0.113.7"), now)
	if err := st.Create(ctx, v4only, func() string { return "k3x7qa" }); err != nil {
		t.Fatal(err)
	}
	dual := &store.Server{Slug: "den", TokenHash: []byte{2}, Port: 443, CreatedAt: now}
	dual.Seen(netip.MustParseAddr("198.51.100.4"), now)
	dual.Seen(netip.MustParseAddr("2a01:e0a::4"), now)
	if err := st.Create(ctx, dual, func() string { return "abcdef" }); err != nil {
		t.Fatal(err)
	}
	if err := st.SetACME(ctx, "k3x7qa", []string{"value-one", "value-two"}, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	s := New(Config{
		Zone:       "getplaid.direct",
		IPv4:       netip.MustParseAddr("192.0.2.53"),
		IPv6:       netip.MustParseAddr("2001:db8::53"),
		AdminEmail: "ops.team@getplaid.tv",
		Records:    st,
		Now:        func() time.Time { return now },
	})
	udp, tcp, err := s.Start("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})
	return &fixture{t: t, udp: udp.String(), tcp: tcp.String(), st: st, now: now, srv: v4only, dual: dual}
}

func (f *fixture) query(name string, qtype uint16) *dns.Msg {
	f.t.Helper()
	return f.queryNet("udp", name, qtype)
}

func (f *fixture) queryNet(network, name string, qtype uint16) *dns.Msg {
	f.t.Helper()
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	m.RecursionDesired = false
	c := &dns.Client{Net: network, Timeout: 2 * time.Second}
	addr := f.udp
	if network == "tcp" {
		addr = f.tcp
	}
	r, _, err := c.Exchange(m, addr)
	if err != nil {
		f.t.Fatalf("%s %s: %v", name, dns.TypeToString[qtype], err)
	}
	return r
}

// values flattens an answer section to comparable strings.
func values(rrs []dns.RR) []string {
	var out []string
	for _, rr := range rrs {
		switch v := rr.(type) {
		case *dns.A:
			out = append(out, v.A.String())
		case *dns.AAAA:
			out = append(out, v.AAAA.String())
		case *dns.TXT:
			out = append(out, v.Txt...)
		case *dns.NS:
			out = append(out, v.Ns)
		case *dns.CAA:
			out = append(out, v.Tag+" "+v.Value)
		case *dns.SOA:
			out = append(out, v.Ns+" "+v.Mbox)
		}
	}
	return out
}

func TestRecords(t *testing.T) {
	f := setup(t)
	tests := []struct {
		name  string
		qtype uint16
		rcode int
		want  []string
		ttl   uint32
	}{
		{"getplaid.direct", dns.TypeSOA, dns.RcodeSuccess, []string{"ns1.getplaid.direct. ops\\.team.getplaid.tv."}, ZoneTTL},
		{"getplaid.direct", dns.TypeNS, dns.RcodeSuccess, []string{"ns1.getplaid.direct.", "ns2.getplaid.direct."}, ZoneTTL},
		{"getplaid.direct", dns.TypeA, dns.RcodeSuccess, []string{"192.0.2.53"}, ZoneTTL},
		{"getplaid.direct", dns.TypeAAAA, dns.RcodeSuccess, []string{"2001:db8::53"}, ZoneTTL},
		{"getplaid.direct", dns.TypeCAA, dns.RcodeSuccess, []string{"issue letsencrypt.org"}, ZoneTTL},
		{"getplaid.direct", dns.TypeMX, dns.RcodeSuccess, nil, 0},
		{"ns1.getplaid.direct", dns.TypeA, dns.RcodeSuccess, []string{"192.0.2.53"}, ZoneTTL},
		{"NS2.GetPlaid.Direct", dns.TypeA, dns.RcodeSuccess, []string{"192.0.2.53"}, ZoneTTL},

		{"living-room-k3x7qa.getplaid.direct", dns.TypeA, dns.RcodeSuccess, []string{"203.0.113.7"}, ServerTTL},
		{"Living-Room-K3X7QA.getplaid.direct", dns.TypeA, dns.RcodeSuccess, []string{"203.0.113.7"}, ServerTTL},
		{"living-room-k3x7qa.getplaid.direct", dns.TypeAAAA, dns.RcodeSuccess, nil, 0},
		{"den-abcdef.getplaid.direct", dns.TypeA, dns.RcodeSuccess, []string{"198.51.100.4"}, ServerTTL},
		{"den-abcdef.getplaid.direct", dns.TypeAAAA, dns.RcodeSuccess, []string{"2a01:e0a::4"}, ServerTTL},
		{"_acme-challenge.living-room-k3x7qa.getplaid.direct", dns.TypeTXT, dns.RcodeSuccess, []string{"value-one", "value-two"}, ServerTTL},
		{"_acme-challenge.den-abcdef.getplaid.direct", dns.TypeTXT, dns.RcodeSuccess, nil, 0},
		{"_acme-challenge.living-room-k3x7qa.getplaid.direct", dns.TypeA, dns.RcodeSuccess, nil, 0},

		// LAN names: private ranges only, only under a registered label.
		{"192-168-1-20.living-room-k3x7qa.getplaid.direct", dns.TypeA, dns.RcodeSuccess, []string{"192.168.1.20"}, ServerTTL},
		{"10-0-0-5.den-abcdef.getplaid.direct", dns.TypeA, dns.RcodeSuccess, []string{"10.0.0.5"}, ServerTTL},
		{"172-20-1-1.den-abcdef.getplaid.direct", dns.TypeA, dns.RcodeSuccess, []string{"172.20.1.1"}, ServerTTL},
		{"100-64-3-4.den-abcdef.getplaid.direct", dns.TypeA, dns.RcodeSuccess, []string{"100.64.3.4"}, ServerTTL},
		{"169-254-3-4.den-abcdef.getplaid.direct", dns.TypeA, dns.RcodeSuccess, []string{"169.254.3.4"}, ServerTTL},
		{"192-168-1-20.living-room-k3x7qa.getplaid.direct", dns.TypeAAAA, dns.RcodeSuccess, nil, 0},
		{"8-8-8-8.living-room-k3x7qa.getplaid.direct", dns.TypeA, dns.RcodeNameError, nil, 0},
		{"172-32-0-1.living-room-k3x7qa.getplaid.direct", dns.TypeA, dns.RcodeNameError, nil, 0},
		{"192-168-01-20.living-room-k3x7qa.getplaid.direct", dns.TypeA, dns.RcodeNameError, nil, 0},
		{"192-168-1-256.living-room-k3x7qa.getplaid.direct", dns.TypeA, dns.RcodeNameError, nil, 0},
		{"192-168-1-20.unknown-zzzzzz.getplaid.direct", dns.TypeA, dns.RcodeNameError, nil, 0},

		// Unknown names.
		{"unknown-zzzzzz.getplaid.direct", dns.TypeA, dns.RcodeNameError, nil, 0},
		{"old-name-k3x7qa.getplaid.direct", dns.TypeA, dns.RcodeNameError, nil, 0}, // slug must match
		{"www.getplaid.direct", dns.TypeA, dns.RcodeNameError, nil, 0},
		{"a.b.living-room-k3x7qa.getplaid.direct", dns.TypeA, dns.RcodeNameError, nil, 0},
		{"_acme-challenge.unknown-zzzzzz.getplaid.direct", dns.TypeTXT, dns.RcodeNameError, nil, 0},

		// Outside the zone: not a resolver.
		{"example.com", dns.TypeA, dns.RcodeRefused, nil, 0},
		{"notgetplaid.direct", dns.TypeA, dns.RcodeRefused, nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name+"/"+dns.TypeToString[tt.qtype], func(t *testing.T) {
			r := f.query(tt.name, tt.qtype)
			if r.Rcode != tt.rcode {
				t.Fatalf("rcode = %s, want %s", dns.RcodeToString[r.Rcode], dns.RcodeToString[tt.rcode])
			}
			if got := values(r.Answer); !slices.Equal(got, tt.want) {
				t.Fatalf("answer = %v, want %v", got, tt.want)
			}
			for _, rr := range r.Answer {
				if rr.Header().Ttl != tt.ttl {
					t.Fatalf("ttl = %d, want %d", rr.Header().Ttl, tt.ttl)
				}
			}
			if tt.rcode == dns.RcodeRefused {
				if r.Authoritative {
					t.Fatal("refused answer must not be authoritative")
				}
				return
			}
			if !r.Authoritative {
				t.Fatal("answer must be authoritative")
			}
			if len(r.Answer) == 0 {
				// NXDOMAIN and NODATA carry the SOA for negative caching.
				if len(r.Ns) != 1 || r.Ns[0].Header().Rrtype != dns.TypeSOA || r.Ns[0].(*dns.SOA).Minttl != NegativeTTL {
					t.Fatalf("authority = %v, want the SOA", r.Ns)
				}
			}
		})
	}
}

func TestNSGlueAndTCP(t *testing.T) {
	f := setup(t)
	r := f.queryNet("tcp", "getplaid.direct", dns.TypeNS)
	if len(r.Answer) != 2 {
		t.Fatalf("answer %v", r.Answer)
	}
	if got := values(r.Extra); !slices.Equal(got, []string{"192.0.2.53", "2001:db8::53", "192.0.2.53", "2001:db8::53"}) {
		t.Fatalf("glue %v", got)
	}
	r = f.queryNet("tcp", "_acme-challenge.living-room-k3x7qa.getplaid.direct", dns.TypeTXT)
	if got := values(r.Answer); !slices.Equal(got, []string{"value-one", "value-two"}) {
		t.Fatalf("tcp txt %v", got)
	}
}

func TestChangesAreLive(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	// Expired ACME values disappear without the janitor.
	srv := New(Config{Zone: "getplaid.direct", Records: f.st, Now: func() time.Time { return f.now.Add(time.Hour) }})
	m := new(dns.Msg)
	m.SetQuestion("_acme-challenge.living-room-k3x7qa.getplaid.direct.", dns.TypeTXT)
	if r := srv.answer(m); len(r.Answer) != 0 || r.Rcode != dns.RcodeSuccess {
		t.Fatalf("expired values served: %v", r)
	}

	// A deleted server is NXDOMAIN at once.
	if err := f.st.Delete(ctx, "k3x7qa"); err != nil {
		t.Fatal(err)
	}
	if r := f.query("living-room-k3x7qa.getplaid.direct", dns.TypeA); r.Rcode != dns.RcodeNameError {
		t.Fatalf("deleted server: %s", dns.RcodeToString[r.Rcode])
	}
}

func TestMbox(t *testing.T) {
	for email, want := range map[string]string{
		"admin@getplaid.tv":    "admin.getplaid.tv.",
		"ops.team@getplaid.tv": `ops\.team.getplaid.tv.`,
		"":                     "hostmaster.getplaid.direct.",
		"nope":                 "hostmaster.getplaid.direct.",
	} {
		if got := mbox(email, "getplaid.direct."); got != want {
			t.Errorf("mbox(%q) = %q, want %q", email, got, want)
		}
	}
}
