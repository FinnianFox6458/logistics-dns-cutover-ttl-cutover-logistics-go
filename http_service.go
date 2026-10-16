package cutover

import (
	"encoding/json"
	"errors"
	"net/http"
)

func Handler(service Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cutover", func(w http.ResponseWriter, r *http.Request) {
		var request CutoverRequest
		if err := decodeJSON(r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		result, err := service.Cutover(r.Context(), request)
		writeResult(w, result, err)
	})
	mux.HandleFunc("POST /rollback", func(w http.ResponseWriter, r *http.Request) {
		var request RollbackRequest
		if err := decodeJSON(r, &request); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		result, err := service.Rollback(r.Context(), request)
		writeResult(w, result, err)
	})
	return mux
}

func decodeJSON(r *http.Request, value any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func writeResult(w http.ResponseWriter, result CutoverResult, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, result)
		return
	}
	status := http.StatusBadGateway
	var validationErr *ValidationError
	var apiErr *InfraiError
	if errors.As(err, &validationErr) {
		status = http.StatusBadRequest
	} else if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		status = apiErr.HTTPStatus
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
