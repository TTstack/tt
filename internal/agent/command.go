package agent

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (s *Server) commandMiddleware(next http.Handler) http.Handler {
	return s.commandJournal.Wrap(next, func(r *http.Request) bool {
		return strings.HasPrefix(r.URL.Path, "/api/control/") || strings.HasPrefix(r.URL.Path, "/api/vms") || strings.HasPrefix(r.URL.Path, "/api/volumes") || strings.HasPrefix(r.URL.Path, "/api/volume-snapshots") || strings.HasPrefix(r.URL.Path, "/api/subjects/") || strings.HasPrefix(r.URL.Path, "/api/images/")
	}, func(r *http.Request, body []byte) (string, error) {
		if strings.HasPrefix(r.URL.Path, "/api/control/") {
			return s.commandJournal.ControlSubject()
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 3 && (parts[1] == "subjects" || parts[1] == "images") {
			return parts[2], nil
		}
		if len(parts) >= 2 && (parts[1] == "volumes" || parts[1] == "volume-snapshots") {
			if len(parts) >= 3 {
				return parts[2], nil
			}
			var spec struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(body, &spec) != nil {
				return "", nil
			}
			return spec.ID, nil
		}

		if len(parts) >= 3 {
			vm, err := ReadVM(r.Context(), s.dbPath, parts[2])
			if err != nil {
				return "", err
			}
			if vm != nil {
				return vm.EnvID, nil
			}
			// Cleanup of an absent VM still honors the explicit subject watermark.
			var envelope struct {
				Subject string `json:"subject"`
			}
			if raw := r.Header.Get("X-Compute-Command"); raw != "" && json.Unmarshal([]byte(raw), &envelope) == nil {
				return envelope.Subject, nil
			}
			return "", nil
		}
		var req struct {
			EnvID string `json:"env_id"`
		}
		if json.Unmarshal(body, &req) != nil {
			return "", nil
		}
		return req.EnvID, nil
	})
}
