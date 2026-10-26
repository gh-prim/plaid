package handlers

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/api"
	"github.com/gh-prim/plaid/server/internal/buildinfo"
)

// GetHealth reports liveness, build info and database status.
func (h *Handlers) GetHealth(ctx context.Context, _ api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	body := api.Health{
		Status:        api.HealthStatusOk,
		Database:      api.HealthDatabaseOk,
		Version:       buildinfo.Version,
		Commit:        buildinfo.Commit,
		UptimeSeconds: int64(buildinfo.Uptime().Seconds()),
	}
	if err := h.db.PingContext(ctx); err != nil {
		body.Status = api.HealthStatusDegraded
		body.Database = api.HealthDatabaseError
		return api.GetHealth503JSONResponse(body), nil
	}
	return api.GetHealth200JSONResponse(body), nil
}
