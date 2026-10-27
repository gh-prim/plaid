package handlers

import (
	"context"
	"errors"
	"testing"

	"github.com/gh-prim/plaid/server/internal/api"
)

type fakePinger struct{ err error }

func (f fakePinger) PingContext(context.Context) error { return f.err }

func TestGetHealthOK(t *testing.T) {
	h := New(Deps{DB: fakePinger{}})
	resp, err := h.GetHealth(context.Background(), api.GetHealthRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	ok, isOK := resp.(api.GetHealth200JSONResponse)
	if !isOK {
		t.Fatalf("got %T, want 200 response", resp)
	}
	if ok.Status != api.HealthStatusOk || ok.Database != api.HealthDatabaseOk {
		t.Errorf("status=%s database=%s, want ok/ok", ok.Status, ok.Database)
	}
	if ok.Version == "" || ok.Commit == "" {
		t.Errorf("version/commit empty: %+v", ok)
	}
	if ok.UptimeSeconds < 0 {
		t.Errorf("uptime = %d", ok.UptimeSeconds)
	}
}

func TestGetHealthDegradedWhenDatabaseFails(t *testing.T) {
	h := New(Deps{DB: fakePinger{err: errors.New("disk on fire")}})
	resp, err := h.GetHealth(context.Background(), api.GetHealthRequestObject{})
	if err != nil {
		t.Fatal(err)
	}
	deg, is503 := resp.(api.GetHealth503JSONResponse)
	if !is503 {
		t.Fatalf("got %T, want 503 response", resp)
	}
	if deg.Status != api.HealthStatusDegraded || deg.Database != api.HealthDatabaseError {
		t.Errorf("status=%s database=%s, want degraded/error", deg.Status, deg.Database)
	}
}
