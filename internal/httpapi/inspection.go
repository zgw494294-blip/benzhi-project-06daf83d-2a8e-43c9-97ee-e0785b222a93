package httpapi

import (
	"net/http"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

type inspectionRequest struct {
	metaRequest
	ID                   string                     `json:"id,omitempty"`
	Role                 string                     `json:"role"`
	InspectorName        string                     `json:"inspectorName"`
	ConfigurationVersion int                        `json:"configurationVersion"`
	CheckItems           []rigging.CheckItem        `json:"checkItems"`
	Findings             []application.FindingInput `json:"findings"`
}

func (a *API) InspectionHandler(w http.ResponseWriter, r *http.Request) {
	var in inspectionRequest
	if err := decode(w, r, &in); err != nil {
		badJSON(w, r, err)
		return
	}
	if len(in.CheckItems) > 30 || len(in.Findings) > 50 {
		fail(w, r, &application.Error{Code: "TOO_MANY_RECORDS", Message: "检查项或问题数量超过限制", Status: 400})
		return
	}
	if err := limit(in.InspectorName, "inspectorName", 80); err != nil {
		fail(w, r, err)
		return
	}
	for _, finding := range in.Findings {
		if err := limit(finding.ID, "findings.id", 160); err != nil {
			fail(w, r, err)
			return
		}
		if err := limit(finding.Description, "findings.description", 2000); err != nil {
			fail(w, r, err)
			return
		}
	}
	c, err := a.service.Inspect(application.SubmitInspection{CommandMeta: in.command(), CaseID: r.PathValue("caseID"), ID: in.ID, Role: in.Role, InspectorName: in.InspectorName, ConfigurationVersion: in.ConfigurationVersion, CheckItems: in.CheckItems, Findings: in.Findings})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, c)
}

type remediationRequest struct {
	metaRequest
	Note           string `json:"note"`
	EvidenceDigest string `json:"evidenceDigest"`
}

func (a *API) RemediationHandler(w http.ResponseWriter, r *http.Request) {
	var in remediationRequest
	if err := decode(w, r, &in); err != nil {
		badJSON(w, r, err)
		return
	}
	if err := limit(in.Note, "note", 2000); err != nil {
		fail(w, r, err)
		return
	}
	if err := limit(in.EvidenceDigest, "evidenceDigest", 500); err != nil {
		fail(w, r, err)
		return
	}
	c, err := a.service.Remediate(application.RemediateFinding{CommandMeta: in.command(), CaseID: r.PathValue("caseID"), FindingID: r.PathValue("findingID"), Note: in.Note, EvidenceDigest: in.EvidenceDigest})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, c)
}

type closeRequest struct {
	metaRequest
	Reviewer string `json:"reviewer"`
}

type reviewRequest struct {
	metaRequest
	Round           int    `json:"round"`
	Decision        string `json:"decision"`
	Reviewer        string `json:"reviewer"`
	RejectionReason string `json:"rejectionReason,omitempty"`
}

func (a *API) ReviewFindingHandler(w http.ResponseWriter, r *http.Request) {
	var in reviewRequest
	if err := decode(w, r, &in); err != nil {
		badJSON(w, r, err)
		return
	}
	for _, field := range []struct {
		value, name string
		max         int
	}{{in.Reviewer, "reviewer", 80}, {in.RejectionReason, "rejectionReason", 1000}} {
		if err := limit(field.value, field.name, field.max); err != nil {
			fail(w, r, err)
			return
		}
	}
	c, err := a.service.ReviewFinding(application.ReviewFinding{CommandMeta: in.command(), CaseID: r.PathValue("caseID"), FindingID: r.PathValue("findingID"), Round: in.Round, Decision: in.Decision, Reviewer: in.Reviewer, RejectionReason: in.RejectionReason})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, c)
}

func (a *API) CloseFindingHandler(w http.ResponseWriter, r *http.Request) {
	var in closeRequest
	if err := decode(w, r, &in); err != nil {
		badJSON(w, r, err)
		return
	}
	c, err := a.service.CloseFinding(application.CloseFinding{CommandMeta: in.command(), CaseID: r.PathValue("caseID"), FindingID: r.PathValue("findingID"), Reviewer: in.Reviewer})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, c)
}
