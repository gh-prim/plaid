package remoteaccess

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/netip"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway2"
	"github.com/jackpal/gateway"
	natpmp "github.com/jackpal/go-nat-pmp"
)

// mappingLease is how long a router keeps a mapping; the service renews
// it every refresh, well before it runs out.
const mappingLease = time.Hour

// Mapper opens a port on the home router.
type Mapper interface {
	// Map forwards external TCP port ext (or another one if the router
	// picks) to internal on this machine and returns the port in use.
	Map(ctx context.Context, internal, ext int) (int, error)
	// Unmap removes the mapping of ext. Best effort.
	Unmap(ctx context.Context, ext int) error
	// Method names the protocol: "natpmp" or "upnp".
	Method() string
}

// Errors a mapper reports, as the codes clients translate.
var (
	errNoGateway      = errors.New("no_gateway")
	errMappingRefused = errors.New("mapping_refused")
)

// DiscoverMapper finds a router that speaks NAT-PMP (quick, one UDP
// round trip) or UPnP IGD (SSDP multicast, a few seconds).
func DiscoverMapper(ctx context.Context) (Mapper, error) {
	if gw, err := gateway.DiscoverGateway(); err == nil {
		c := natpmp.NewClientWithTimeout(gw, 2*time.Second)
		if _, err := c.GetExternalAddress(); err == nil {
			return &natPMP{client: c}, nil
		}
	}
	if m, err := discoverUPnP(ctx); err == nil {
		return m, nil
	}
	return nil, errNoGateway
}

type natPMP struct{ client *natpmp.Client }

func (n *natPMP) Method() string { return "natpmp" }

func (n *natPMP) Map(_ context.Context, internal, ext int) (int, error) {
	res, err := n.client.AddPortMapping("tcp", internal, ext, int(mappingLease.Seconds()))
	if err != nil {
		return 0, fmt.Errorf("%w: %w", errMappingRefused, err)
	}
	return int(res.MappedExternalPort), nil
}

func (n *natPMP) Unmap(_ context.Context, ext int) error {
	// RFC 6886: lifetime 0 deletes; the internal port names the mapping,
	// but routers accept the external one we asked for.
	_, err := n.client.AddPortMapping("tcp", ext, 0, 0)
	return err
}

// upnp maps through one of the three IGD service flavours routers offer.
type upnp struct {
	add     func(ctx context.Context, ext, internal int, client string) error
	del     func(ctx context.Context, ext int) error
	localIP string
}

func (u *upnp) Method() string { return "upnp" }

func (u *upnp) Map(ctx context.Context, internal, ext int) (int, error) {
	// A router that already maps ext to someone else refuses it
	// (ConflictInMappingEntry): try a few random high ports.
	try := []int{ext}
	for range 4 {
		try = append(try, 40000+rand.IntN(20000)) //nolint:gosec // not a secret
	}
	var last error
	for _, p := range try {
		if last = u.add(ctx, p, internal, u.localIP); last == nil {
			return p, nil
		}
	}
	return 0, fmt.Errorf("%w: %w", errMappingRefused, last)
}

func (u *upnp) Unmap(ctx context.Context, ext int) error { return u.del(ctx, ext) }

func discoverUPnP(ctx context.Context) (*upnp, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if c, _, err := internetgateway2.NewWANIPConnection2ClientsCtx(ctx); err == nil && len(c) > 0 {
		return fromIGD(c[0].Location.Host, c[0])
	}
	if c, _, err := internetgateway2.NewWANIPConnection1ClientsCtx(ctx); err == nil && len(c) > 0 {
		return fromIGD(c[0].Location.Host, c[0])
	}
	if c, _, err := internetgateway2.NewWANPPPConnection1ClientsCtx(ctx); err == nil && len(c) > 0 {
		return fromIGD(c[0].Location.Host, c[0])
	}
	return nil, errNoGateway
}

// igd is what the three UPnP service flavours have in common.
type igd interface {
	AddPortMappingCtx(ctx context.Context, remoteHost string, externalPort uint16, protocol string, internalPort uint16, internalClient string, enabled bool, description string, leaseDuration uint32) error
	DeletePortMappingCtx(ctx context.Context, remoteHost string, externalPort uint16, protocol string) error
}

func fromIGD(routerHost string, cl igd) (*upnp, error) {
	return newUPnP(routerHost,
		func(ctx context.Context, ext, in int, ip string) error {
			return cl.AddPortMappingCtx(ctx, "", uint16(ext), "TCP", uint16(in), ip, true, "PLAID", uint32(mappingLease.Seconds())) //nolint:gosec // ports are 1..65535
		},
		func(ctx context.Context, ext int) error {
			return cl.DeletePortMappingCtx(ctx, "", uint16(ext), "TCP") //nolint:gosec // ports are 1..65535
		})
}

// newUPnP finds the local address the router sees us at: the mapping
// must name this machine's LAN IP.
func newUPnP(routerHost string, add func(context.Context, int, int, string) error, del func(context.Context, int) error) (*upnp, error) {
	host, _, err := net.SplitHostPort(routerHost)
	if err != nil {
		host = routerHost
	}
	ip, err := localIPToward(host)
	if err != nil {
		return nil, err
	}
	return &upnp{add: add, del: del, localIP: ip}, nil
}

// localIPToward returns the source address the kernel would use to reach
// host. Dialing UDP sends nothing.
func localIPToward(host string) (string, error) {
	conn, err := (&net.Dialer{}).DialContext(context.Background(), "udp", net.JoinHostPort(host, "1900"))
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()
	addr, err := netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		return "", err
	}
	return addr.Addr().Unmap().String(), nil
}
