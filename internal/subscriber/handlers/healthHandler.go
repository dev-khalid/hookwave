package handlers

import (
	"encoding/json"
	"net/http"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	type ServerStatusType string

	const (
		StatusOk    ServerStatusType = "OK"
		StatusError ServerStatusType = "ERROR"
	)

	type HealthStruct struct {
		Message string           `json:"message"`
		Status  ServerStatusType `json:"status"`
		// Extend this struct with more fields as needed, e.g., version, uptime, s3 reachability etc.
	}

	w.WriteHeader(http.StatusOK)

	response := HealthStruct{
		Message: "Subscriber service is healthy",
		Status:  StatusOk,
	}

	json.NewEncoder(w).Encode(response)

}
