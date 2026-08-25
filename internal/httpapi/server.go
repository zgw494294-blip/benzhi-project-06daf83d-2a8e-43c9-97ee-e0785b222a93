package httpapi

import (
	"net/http"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
)

type API struct {
	service *application.Service
	web     http.Handler
}

func New(service *application.Service, web http.Handler) http.Handler {
	a := &API{service: service, web: web}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.HealthHandler)
	mux.HandleFunc("GET /api/v1/cases", a.ListCasesHandler)
	mux.HandleFunc("POST /api/v1/cases", a.CreateCaseHandler)
	mux.HandleFunc("GET /api/v1/cases/{caseID}", a.CaseDetailHandler)
	mux.HandleFunc("PUT /api/v1/cases/{caseID}/configuration", a.ConfigurationHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/configuration/preflight", a.ConfigurationPreflightHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/inspections", a.InspectionHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/findings/{findingID}/remediation", a.RemediationHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/findings/{findingID}/close", a.CloseFindingHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/findings/{findingID}/review", a.ReviewFindingHandler)
	mux.HandleFunc("PUT /api/v1/cases/{caseID}/trial-standard", a.TrialStandardHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/trial-lifts", a.TrialLiftHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/freeze", a.FreezeHandler)
	mux.HandleFunc("POST /api/v1/cases/{caseID}/credentials", a.IssueCredentialHandler)
	mux.HandleFunc("GET /api/v1/credentials/{number}", a.VerifyCredentialHandler)
	mux.Handle("/", web)
	return middleware(mux)
}
func (a *API) HealthHandler(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, map[string]string{"status": "ok"})
}
