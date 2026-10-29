package handlers

import (
	"context"

	"github.com/gh-prim/plaid/server/internal/api"
	problemfeed "github.com/gh-prim/plaid/server/internal/problems"
)

// ListProblems lists what needs the admin's attention (F007 R29).
func (h *Handlers) ListProblems(ctx context.Context, _ api.ListProblemsRequestObject) (api.ListProblemsResponseObject, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	out := api.ProblemList{Problems: []api.Problem{}}
	if h.problems == nil {
		return api.ListProblems200JSONResponse(out), nil
	}
	list, err := h.problems.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		out.Problems = append(out.Problems, toProblem(p))
	}
	return api.ListProblems200JSONResponse(out), nil
}

func toProblem(p problemfeed.Problem) api.Problem {
	return api.Problem{
		Id: p.ID, Code: api.ProblemCode(p.Code), Params: p.Params,
		FirstSeen: p.FirstSeen, LastSeen: p.LastSeen, Count: p.Count, ResolvedAt: p.ResolvedAt,
	}
}
