package httpapi

import (
	"net/http"
	"time"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

type trialRequest struct {
	metaRequest
	ID             string                     `json:"id,omitempty"`
	OperatorName   string                     `json:"operatorName"`
	StartedAt      time.Time                  `json:"startedAt"`
	DeadlineAt     time.Time                  `json:"deadlineAt"`
	Stages         []rigging.StageObservation `json:"stageObservations"`
	Anomalies      []string                   `json:"anomalies"`
	StandardDigest string                     `json:"standardDigest,omitempty"`
}

type trialStandardRequest struct {
	metaRequest
	Stages              []rigging.TrialStageCriterion `json:"stages"`
	AllowedReboundMM    int64                         `json:"allowedReboundMm"`
	MaxTotalDurationSec int                           `json:"maxTotalDurationSec"`
}

func (a *API) TrialStandardHandler(w http.ResponseWriter, r *http.Request) {
	var in trialStandardRequest
	if err := decode(w, r, &in); err != nil {
		badJSON(w, r, err)
		return
	}
	if len(in.Stages) > 4 {
		fail(w, r, &application.Error{Code: "TOO_MANY_RECORDS", Message: "试吊标准阶段数量超过限制", Status: 400})
		return
	}
	c, err := a.service.ConfirmTrialStandard(application.ConfirmTrialStandard{CommandMeta: in.command(), CaseID: r.PathValue("caseID"), Stages: in.Stages, AllowedReboundMM: in.AllowedReboundMM, MaxTotalDurationSec: in.MaxTotalDurationSec})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, c)
}

func (a *API) TrialLiftHandler(w http.ResponseWriter, r *http.Request) {
	var in trialRequest
	if err := decode(w, r, &in); err != nil {
		badJSON(w, r, err)
		return
	}
	if len(in.Stages) > 4 || len(in.Anomalies) > 20 {
		fail(w, r, &application.Error{Code: "TOO_MANY_RECORDS", Message: "试吊阶段或异常数量超过限制", Status: 400})
		return
	}
	if err := limit(in.OperatorName, "operatorName", 80); err != nil {
		fail(w, r, err)
		return
	}
	for _, stage := range in.Stages {
		if err := limit(stage.Note, "stageObservations.note", 500); err != nil {
			fail(w, r, err)
			return
		}
	}
	for _, anomaly := range in.Anomalies {
		if err := limit(anomaly, "anomalies", 500); err != nil {
			fail(w, r, err)
			return
		}
	}
	c, err := a.service.RecordTrial(application.RecordTrial{CommandMeta: in.command(), CaseID: r.PathValue("caseID"), ID: in.ID, OperatorName: in.OperatorName, StartedAt: in.StartedAt, DeadlineAt: in.DeadlineAt, Stages: in.Stages, Anomalies: in.Anomalies, StandardDigest: in.StandardDigest})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, c)
}
func (a *API) FreezeHandler(w http.ResponseWriter, r *http.Request) {
	var in metaRequest
	if err := decode(w, r, &in); err != nil {
		badJSON(w, r, err)
		return
	}
	c, err := a.service.Freeze(application.FreezeManifest{CommandMeta: in.command(), CaseID: r.PathValue("caseID")})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, c)
}

type issueRequest struct {
	metaRequest
	IssuedBy string `json:"issuedBy"`
}

func (a *API) IssueCredentialHandler(w http.ResponseWriter, r *http.Request) {
	var in issueRequest
	if err := decode(w, r, &in); err != nil {
		badJSON(w, r, err)
		return
	}
	c, err := a.service.Issue(application.IssueCredential{CommandMeta: in.command(), CaseID: r.PathValue("caseID"), IssuedBy: in.IssuedBy})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 201, c)
}
func (a *API) VerifyCredentialHandler(w http.ResponseWriter, r *http.Request) {
	result, err := a.service.Verify(r.PathValue("number"))
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, result)
}
