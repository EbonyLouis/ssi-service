package dwn

import (
	"bytes"
	"github.com/TBD54566975/ssi-sdk/credential/manifest"
	"github.com/goccy/go-json"
	"github.com/tbd54566975/ssi-service/internal/util"
	"io"
	"net/http"
)

type DWNPublishManifestRequest struct {
	Manifest manifest.CredentialManifest `json:"manifest" validate:"required"`
}

type DWNPublishManifestResponse struct {
	Status   int    `json:"status" validate:"required"`
	Response string `json:"response" validate:"required"`
}

// PublishManifest publishes a CredentialManifest to a DWN
func PublishManifest(endpoint string, manifest manifest.CredentialManifest) (*DWNPublishManifestResponse, error) {

	dwnReq := DWNPublishManifestRequest{Manifest: manifest}
	postResp, err := Post(endpoint, dwnReq)

	if err != nil {
		return nil, util.LoggingErrorMsg(err, "problem with posting to dwn")
	}

	defer postResp.Body.Close()

	b, _ := io.ReadAll(postResp.Body)
	body := string(b)

	return &DWNPublishManifestResponse{Status: postResp.StatusCode, Response: body}, nil

}

// Post does a post request with data to provided endpoint
func Post(endpoint string, data interface{}) (*http.Response, error) {
	// convert response payload to json
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))

	if err != nil {
		return nil, err
	}

	return resp, nil
}
