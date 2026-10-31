// Package dnsserver is the authoritative name server for the zone: the apex,
// ns1/ns2, and one set of records per registered server.
package dnsserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/gh-prim/plaid/services/direct/internal/slug"
	"github.com/gh-prim/plaid/services/direct/internal/store"
)

const (
	// ServerTTL is short so a new public IP propagates within a minute.
	ServerTTL = 60
	// ZoneTTL covers the zone's own records, which rarely change.
	ZoneTTL = 3600
	// NegativeTTL is the SOA minimum: how long resolvers cache NXDOMAIN. A
	// freshly registered name must resolve quickly.
	NegativeTTL = 60
)

// Records is what the DNS server reads from the database.
type Records interface {
	Get(ctx context.Context, id string) (store.Server, error)
	ACMEValues(ctx context.Context, id string, now time.Time) ([]string, error)
}

// Config describes the zone.
type Config struct {
	Zone       string     // e.g. getplaid.direct
	IPv4       netip.Addr // the service's own address: apex, ns1, ns2
	IPv6       netip.Addr // optional
	AdminEmail string     // SOA RNAME
	Records    Records
	Logger     *slog.Logger
	Now        func() time.Time
}

// Server answers DNS queries over UDP and TCP.
type Server struct {
	cfg    Config
	zone   string // fully qualified, lowercase, with the trailing dot
	serial uint32
	log    *slog.Logger

	udp, tcp *dns.Server
}

// New prepares a server; call Start to listen.
func New(cfg Config) *Server {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	return &Server{
		cfg:  cfg,
		zone: dns.Fqdn(strings.ToLower(cfg.Zone)),
		// Nothing transfers the zone, so the serial only has to move forward
		// across restarts: the start time does that.
		serial: uint32(cfg.Now().Unix()), //nolint:gosec // fits until 2106
		log:    cfg.Logger,
	}
}

// Start listens on addr over UDP and TCP (same port) and serves in the
// background. It returns the bound addresses, useful when addr's port is 0.
func (s *Server) Start(addr string) (udpAddr, tcpAddr net.Addr, err error) {
	var lc net.ListenConfig
	ctx := context.Background()
	pc, err := lc.ListenPacket(ctx, "udp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("dns udp listen: %w", err)
	}
	host, _, _ := net.SplitHostPort(addr)
	port := pc.LocalAddr().(*net.UDPAddr).Port
	l, err := lc.Listen(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		_ = pc.Close()
		return nil, nil, fmt.Errorf("dns tcp listen: %w", err)
	}
	s.udp = &dns.Server{PacketConn: pc, Handler: s}
	s.tcp = &dns.Server{Listener: l, Handler: s, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	for _, srv := range []*dns.Server{s.udp, s.tcp} {
		go func() {
			if err := srv.ActivateAndServe(); err != nil {
				s.log.Error("dns server stopped", "err", err)
			}
		}()
	}
	return pc.LocalAddr(), l.Addr(), nil
}

// Shutdown stops both listeners.
func (s *Server) Shutdown(ctx context.Context) error {
	var errs []error
	for _, srv := range []*dns.Server{s.udp, s.tcp} {
		if srv != nil {
			errs = append(errs, srv.ShutdownContext(ctx))
		}
	}
	return errors.Join(errs...)
}

// ServeDNS implements dns.Handler.
func (s *Server) ServeDNS(w dns.ResponseWriter, req *dns.Msg) {
	m := s.answer(req)
	if _, udp := w.RemoteAddr().(*net.UDPAddr); udp {
		size := dns.MinMsgSize
		if opt := req.IsEdns0(); opt != nil {
			size = int(opt.UDPSize())
		}
		m.Truncate(size)
	}
	if err := w.WriteMsg(m); err != nil {
		s.log.Debug("dns write failed", "err", err)
	}
}

func (s *Server) answer(req *dns.Msg) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(req)
	m.RecursionAvailable = false
	if opt := req.IsEdns0(); opt != nil {
		m.SetEdns0(dns.DefaultMsgSize, false)
	}
	if req.Opcode != dns.OpcodeQuery {
		m.Rcode = dns.RcodeNotImplemented
		return m
	}
	if len(req.Question) != 1 {
		m.Rcode = dns.RcodeFormatError
		return m
	}
	q := req.Question[0]
	name := strings.ToLower(q.Name)
	if q.Qclass != dns.ClassINET || !dns.IsSubDomain(s.zone, name) {
		m.Rcode = dns.RcodeRefused // not our zone: we are not a resolver
		return m
	}
	m.Authoritative = true

	rel := strings.TrimSuffix(strings.TrimSuffix(name, s.zone), ".")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	rrs, exists, err := s.lookup(ctx, rel, q)
	if err != nil {
		s.log.Error("dns lookup failed", "name", name, "err", err)
		m.Authoritative = false
		m.Rcode = dns.RcodeServerFailure
		return m
	}
	switch {
	case !exists:
		m.Rcode = dns.RcodeNameError
		m.Ns = []dns.RR{s.soa()}
	case len(rrs) == 0:
		m.Ns = []dns.RR{s.soa()} // NODATA
	default:
		m.Answer = rrs
		if q.Qtype == dns.TypeNS && rel == "" {
			m.Extra = append(m.Extra, s.glue()...)
		}
	}
	return m
}

// lookup returns the answer for q at rel (the name relative to the zone,
// "" for the apex) and whether the name exists at all.
func (s *Server) lookup(ctx context.Context, rel string, q dns.Question) ([]dns.RR, bool, error) {
	hdr := func(t uint16, ttl uint32) dns.RR_Header {
		return dns.RR_Header{Name: q.Name, Rrtype: t, Class: dns.ClassINET, Ttl: ttl}
	}
	addrs := func(v4, v6 netip.Addr, ttl uint32) []dns.RR {
		switch {
		case q.Qtype == dns.TypeA && v4.IsValid():
			return []dns.RR{&dns.A{Hdr: hdr(dns.TypeA, ttl), A: v4.AsSlice()}}
		case q.Qtype == dns.TypeAAAA && v6.IsValid():
			return []dns.RR{&dns.AAAA{Hdr: hdr(dns.TypeAAAA, ttl), AAAA: v6.AsSlice()}}
		}
		return nil
	}

	switch rel {
	case "":
		switch q.Qtype {
		case dns.TypeSOA:
			return []dns.RR{s.soa()}, true, nil
		case dns.TypeNS:
			return []dns.RR{
				&dns.NS{Hdr: hdr(dns.TypeNS, ZoneTTL), Ns: "ns1." + s.zone},
				&dns.NS{Hdr: hdr(dns.TypeNS, ZoneTTL), Ns: "ns2." + s.zone},
			}, true, nil
		case dns.TypeCAA:
			return []dns.RR{&dns.CAA{Hdr: hdr(dns.TypeCAA, ZoneTTL), Flag: 0, Tag: "issue", Value: "letsencrypt.org"}}, true, nil
		}
		return addrs(s.cfg.IPv4, s.cfg.IPv6, ZoneTTL), true, nil
	case "ns1", "ns2":
		return addrs(s.cfg.IPv4, s.cfg.IPv6, ZoneTTL), true, nil
	}

	parts := strings.Split(rel, ".")
	if len(parts) > 2 {
		return nil, false, nil
	}
	label := parts[len(parts)-1]
	srv, ok, err := s.server(ctx, label)
	if err != nil || !ok {
		return nil, false, err
	}
	if len(parts) == 1 {
		var v4, v6 netip.Addr
		if a, ok := srv.A(); ok {
			v4 = a
		}
		if a, ok := srv.AAAA(); ok {
			v6 = a
		}
		return addrs(v4, v6, ServerTTL), true, nil
	}

	switch sub := parts[0]; sub {
	case "_acme-challenge":
		if q.Qtype != dns.TypeTXT {
			return nil, true, nil
		}
		values, err := s.cfg.Records.ACMEValues(ctx, srv.ID, s.cfg.Now())
		if err != nil {
			return nil, false, err
		}
		rrs := make([]dns.RR, 0, len(values))
		for _, v := range values {
			rrs = append(rrs, &dns.TXT{Hdr: hdr(dns.TypeTXT, ServerTTL), Txt: []string{v}})
		}
		return rrs, true, nil
	default:
		ip, ok := lanIP(sub)
		if !ok {
			return nil, false, nil
		}
		return addrs(ip, netip.Addr{}, ServerTTL), true, nil
	}
}

// server finds the registered server for label; the slug must match the
// current one, so a renamed server's old hostname stops resolving.
func (s *Server) server(ctx context.Context, label string) (store.Server, bool, error) {
	sl, id, ok := slug.Parse(label)
	if !ok {
		return store.Server{}, false, nil
	}
	srv, err := s.cfg.Records.Get(ctx, id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && srv.Slug != sl) {
		return store.Server{}, false, nil
	}
	if err != nil {
		return store.Server{}, false, err
	}
	return srv, true, nil
}

var lanRanges = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("100.64.0.0/10"),  // CGNAT
	netip.MustParsePrefix("169.254.0.0/16"), // link-local
}

// lanIP parses "a-b-c-d" into a.b.c.d when it is a private address. Public
// addresses are refused: the zone would otherwise point anywhere, for anyone.
func lanIP(s string) (netip.Addr, bool) {
	octets := strings.Split(s, "-")
	if len(octets) != 4 {
		return netip.Addr{}, false
	}
	var b [4]byte
	for i, o := range octets {
		// No leading zeros or signs: one spelling per address.
		if o == "" || len(o) > 3 || (len(o) > 1 && o[0] == '0') {
			return netip.Addr{}, false
		}
		n, err := strconv.Atoi(o)
		if err != nil || n < 0 || n > 255 {
			return netip.Addr{}, false
		}
		b[i] = byte(n)
	}
	ip := netip.AddrFrom4(b)
	for _, p := range lanRanges {
		if p.Contains(ip) {
			return ip, true
		}
	}
	return netip.Addr{}, false
}

func (s *Server) soa() dns.RR {
	return &dns.SOA{
		Hdr:     dns.RR_Header{Name: s.zone, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: ZoneTTL},
		Ns:      "ns1." + s.zone,
		Mbox:    mbox(s.cfg.AdminEmail, s.zone),
		Serial:  s.serial,
		Refresh: 3600,
		Retry:   600,
		Expire:  604800,
		Minttl:  NegativeTTL,
	}
}

// mbox turns admin@example.com into admin.example.com., escaping dots in
// the local part as RFC 1035 requires.
func mbox(email, zone string) string {
	local, domain, ok := strings.Cut(email, "@")
	if !ok || local == "" || domain == "" {
		return "hostmaster." + zone
	}
	return strings.ReplaceAll(local, ".", `\.`) + "." + dns.Fqdn(domain)
}

func (s *Server) glue() []dns.RR {
	var rrs []dns.RR
	for _, ns := range []string{"ns1." + s.zone, "ns2." + s.zone} {
		if s.cfg.IPv4.IsValid() {
			rrs = append(rrs, &dns.A{Hdr: dns.RR_Header{Name: ns, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ZoneTTL}, A: s.cfg.IPv4.AsSlice()})
		}
		if s.cfg.IPv6.IsValid() {
			rrs = append(rrs, &dns.AAAA{Hdr: dns.RR_Header{Name: ns, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ZoneTTL}, AAAA: s.cfg.IPv6.AsSlice()})
		}
	}
	return rrs
}
