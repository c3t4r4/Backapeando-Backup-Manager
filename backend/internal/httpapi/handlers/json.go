package handlers

import (
	"encoding/json"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// maxRequestBodyBytes caps how much of a request body readJSON will
// consume. Nothing this API accepts (server/storage-target/retention-policy
// fields) is anywhere near this size — an unauthenticated client (login is
// public) sending a multi-GB body should be rejected cheaply instead of
// forcing the process to buffer it.
const maxRequestBodyBytes = 1 << 20 // 1 MiB

func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
