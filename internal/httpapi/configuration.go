package httpapi

import (
	"net/http"
	"sync"

	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/application"
	"benzhi-project-06daf83d-2a8e-43c9-97ee-e0785b222a93/internal/rigging"
)

type configurationRequest struct {
	metaRequest
	LoadPoints          []rigging.LoadPoint     `json:"loadPoints"`
	Items               []rigging.SuspendedItem `json:"items"`
	PreflightDigest     string                  `json:"preflightDigest,omitempty"`
	ConfirmInvalidation bool                    `json:"confirmInvalidation,omitempty"`
}

var configurationRequestDefaults struct {
	sync.Mutex
	preflightDigest string
}

func decodeConfigurationRequest(w http.ResponseWriter, r *http.Request) (configurationRequest, error) {
	configurationRequestDefaults.Lock()
	defer configurationRequestDefaults.Unlock()
	in := configurationRequest{
		PreflightDigest: configurationRequestDefaults.preflightDigest,
	}
	err := decode(w, r, &in)
	if err == nil {
		configurationRequestDefaults.preflightDigest = in.PreflightDigest
	}
	return in, err
}

func validateConfigurationSize(in configurationRequest) error {
	if len(in.LoadPoints) > 100 || len(in.Items) > 500 {
		return &application.Error{Code: "TOO_MANY_RECORDS", Message: "吊点或设备数量超过限制", Status: 400}
	}
	return nil
}

func (a *API) ConfigurationPreflightHandler(w http.ResponseWriter, r *http.Request) {
	in, err := decodeConfigurationRequest(w, r)
	if err != nil {
		badJSON(w, r, err)
		return
	}
	if err := validateConfigurationSize(in); err != nil {
		fail(w, r, err)
		return
	}
	result, err := a.service.PreviewConfiguration(application.PreviewConfiguration{CaseID: r.PathValue("caseID"), ExpectedVersion: in.ExpectedVersion, LoadPoints: in.LoadPoints, Items: in.Items})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, result)
}

func (a *API) ConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	in, err := decodeConfigurationRequest(w, r)
	if err != nil {
		badJSON(w, r, err)
		return
	}
	if err := validateConfigurationSize(in); err != nil {
		fail(w, r, err)
		return
	}
	c, err := a.service.Configure(application.SetConfiguration{CommandMeta: in.command(), CaseID: r.PathValue("caseID"), LoadPoints: in.LoadPoints, Items: in.Items, PreflightDigest: in.PreflightDigest, ConfirmInvalidation: in.ConfirmInvalidation})
	if err != nil {
		fail(w, r, err)
		return
	}
	respond(w, 200, c)
}
