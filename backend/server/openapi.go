package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

// openapiYAML is the public api's spec, written by hand. TestSpecCoversRoutes
// fails when a public route is missing from it.
//
//go:embed openapi.yaml
var openapiYAML []byte

var openapiJSON = sync.OnceValues(func() ([]byte, error) {
	var doc any
	if err := yaml.Unmarshal(openapiYAML, &doc); err != nil {
		return nil, fmt.Errorf("openapi.yaml: %w", err)
	}
	return json.MarshalIndent(doc, "", "  ")
})

func (s *Server) getOpenAPIYAML(w http.ResponseWriter, r *http.Request) {
	writeCachedBytes(w, r, "application/yaml; charset=utf-8", openapiYAML, time.Hour)
}

func (s *Server) getOpenAPIJSON(w http.ResponseWriter, r *http.Request) {
	b, err := openapiJSON()
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeCachedBytes(w, r, "application/json; charset=utf-8", b, time.Hour)
}
