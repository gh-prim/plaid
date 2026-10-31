package remoteaccess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultDirectURL is the getplaid.direct API (services/direct/PROTOCOL.md).
const DefaultDirectURL = "https://getplaid.direct/api/v1"

// Registration is what getplaid.direct answers about this server.
type Registration struct {
	ID       string `json:"id"`
	Token    string `json:"token,omitempty"` // only on registration
	Hostname string `json:"hostname"`
	URL      string `json:"url"`
	ShortURL string `json:"shortUrl"`
	PublicIP string `json:"publicIp"`
	Port     int    `json:"port"`
}

// Reachability is getplaid.direct's check of the server from outside.
type Reachability struct {
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

// Errors the client reports, as the codes clients translate.
var (
	errDirectUnreachable = errors.New("direct_unreachable")
	errRateLimited       = errors.New("rate_limited")
	errUnauthorized      = errors.New("unauthorized")
)

// Directory is the getplaid.direct API, faked in tests.
type Directory interface {
	Register(ctx context.Context, name string, port int, version string) (Registration, error)
	Update(ctx context.Context, id, token, name string, port int, version string) (Registration, error)
	Unregister(ctx context.Context, id, token string) error
	SetChallenge(ctx context.Context, id, token string, values []string) error
	ClearChallenge(ctx context.Context, id, token string) error
	Reachability(ctx context.Context, id, token string) (Reachability, error)
}

// DirectClient talks to getplaid.direct over HTTPS.
type DirectClient struct {
	BaseURL string
	HTTP    *http.Client
}

// NewDirectClient returns a client of the API at baseURL.
func NewDirectClient(baseURL string) *DirectClient {
	return &DirectClient{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 20 * time.Second}}
}

type serverBody struct {
	Name    string `json:"name"`
	Port    int    `json:"port"`
	Version string `json:"version"`
}

// Register asks for a hostname; the token in the answer is shown once.
func (c *DirectClient) Register(ctx context.Context, name string, port int, version string) (Registration, error) {
	var r Registration
	err := c.do(ctx, http.MethodPost, "/servers", "", serverBody{name, port, version}, &r)
	return r, err
}

// Update is the heartbeat: it refreshes the public IP, port and name.
func (c *DirectClient) Update(ctx context.Context, id, token, name string, port int, version string) (Registration, error) {
	var r Registration
	err := c.do(ctx, http.MethodPut, "/servers/"+id, token, serverBody{name, port, version}, &r)
	return r, err
}

// Unregister removes the hostname at once.
func (c *DirectClient) Unregister(ctx context.Context, id, token string) error {
	return c.do(ctx, http.MethodDelete, "/servers/"+id, token, nil, nil)
}

// SetChallenge publishes the DNS-01 values of a certificate order.
func (c *DirectClient) SetChallenge(ctx context.Context, id, token string, values []string) error {
	return c.do(ctx, http.MethodPut, "/servers/"+id+"/acme-challenge", token, map[string][]string{"values": values}, nil)
}

// ClearChallenge removes them once the order is done.
func (c *DirectClient) ClearChallenge(ctx context.Context, id, token string) error {
	return c.do(ctx, http.MethodDelete, "/servers/"+id+"/acme-challenge", token, nil, nil)
}

// Reachability asks getplaid.direct to open the server's public address.
func (c *DirectClient) Reachability(ctx context.Context, id, token string) (Reachability, error) {
	var r Reachability
	err := c.do(ctx, http.MethodPost, "/servers/"+id+"/reachability", token, nil, &r)
	return r, err
}

func (c *DirectClient) do(ctx context.Context, method, path, token string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", errDirectUnreachable, err)
	}
	defer func() { _ = res.Body.Close() }()
	switch {
	case res.StatusCode == http.StatusUnauthorized:
		return errUnauthorized
	case res.StatusCode == http.StatusTooManyRequests:
		return errRateLimited
	case res.StatusCode >= 300:
		msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("%w: %s %s: %d %s", errDirectUnreachable, method, path, res.StatusCode, bytes.TrimSpace(msg))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("%w: decode %s: %w", errDirectUnreachable, path, err)
	}
	return nil
}
