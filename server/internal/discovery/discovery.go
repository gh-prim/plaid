// Package discovery advertises the server on the local network over
// Bonjour (mDNS/DNS-SD), so TV apps find it without an address typed in.
package discovery

import (
	"log/slog"

	"github.com/libp2p/zeroconf/v2"
)

// ServiceType is the DNS-SD type TV apps browse for.
const ServiceType = "_plaid._tcp"

// Options says what to announce.
type Options struct {
	// ID is a stable id of this server, so a client can tell two apart.
	ID      string
	Name    string
	Version string
	Port    int
	Log     *slog.Logger
}

// Advertise announces the server until Shutdown is called on the result.
// A failure (no multicast interface, port taken) is logged, never fatal:
// clients can still type the address.
func Advertise(o Options) *zeroconf.Server {
	txt := []string{"id=" + o.ID, "name=" + o.Name, "version=" + o.Version, "path=/api/v1"}
	srv, err := zeroconf.Register(o.Name, ServiceType, "local.", o.Port, txt, nil)
	if err != nil {
		o.Log.Warn("bonjour: not advertising the server on the local network", "code", "bonjour_failed", "err", err)
		return nil
	}
	o.Log.Info("bonjour: advertising the server", "type", ServiceType, "name", o.Name, "port", o.Port)
	return srv
}
