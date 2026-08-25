package httpapi

import (
	"net/http"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
)

type metaRequest struct {
	ExpectedVersion int    `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
}

func (m metaRequest) command() application.CommandMeta {
	return application.CommandMeta{ExpectedVersion: m.ExpectedVersion, IdempotencyKey: m.IdempotencyKey, Actor: m.Actor}
}

type createRequest struct {
	metaRequest
	ID            string    `json:"id,omitempty"`
	Title         string    `json:"title"`
	Venue         string    `json:"venue"`
	PerformanceAt time.Time `json:"performanceAt"`
	ManagerName   string    `json:"managerName"`
}

func (a *API) CreateCaseHandler(w http.ResponseWriter, r *http.Request) {
	var in createRequest
	if err := decode(w, r, &in); err != nil {
		badJSON(w, r, err)
		return
	}
	for _, field := range []struct {
		v, n string
		m    int
	}{{in.Title, "title", 120}, {in.Venue, "venue", 120}, {in.ManagerName, "managerName", 80}, {in.Actor, "actor", 80}, {in.IdempotencyKey, "idempotencyKey", 160}} {
		if err := limit(field.v, field.n, field.m); err != nil {
			fail(w, r, err)
			return
		}
	}
	c, err := a.service.Create(application.CreateCase{CommandMeta: in.command(), ID: in.ID, Title: in.Title, Venue: in.Venue, PerformanceAt: in.PerformanceAt, ManagerName: in.ManagerName})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 201, c)
}
func (a *API) ListCasesHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := application.CaseListFilter{Status: query.Get("status"), Venue: query.Get("venue"), RiskLevel: query.Get("riskLevel")}
	if err := limit(filter.Venue, "venue", 120); err != nil {
		fail(w, r, err)
		return
	}
	parseTime := func(name string) (*time.Time, error) {
		value := query.Get(name)
		if value == "" {
			return nil, nil
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, &application.Error{Code: "INVALID_TIME_FILTER", Message: name + " 必须为 RFC3339 时间", Status: 400}
		}
		parsed = parsed.UTC()
		return &parsed, nil
	}
	var err error
	if filter.PerformanceFrom, err = parseTime("performanceFrom"); err != nil {
		fail(w, r, err)
		return
	}
	if filter.PerformanceTo, err = parseTime("performanceTo"); err != nil {
		fail(w, r, err)
		return
	}
	dashboard, err := a.service.Dashboard(filter)
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, dashboard)
}
func (a *API) CaseDetailHandler(w http.ResponseWriter, r *http.Request) {
	detail, err := a.service.Detail(r.PathValue("caseID"))
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, detail)
}
