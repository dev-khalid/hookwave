package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/dev-khalid/hookwave/internal/events"
	"github.com/dev-khalid/hookwave/internal/events/order"
	"github.com/dev-khalid/hookwave/internal/utility"
)

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	_, event, err := events.Decode(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := order.Validate.Struct(event); err != nil {
		formattedErrors := utility.FormatValidationErrors(err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(formattedErrors)
		return
	}

	// Store it to s3

	var requestBody map[string]interface{}
	if err := json.Unmarshal(body, &requestBody); err != nil {
		http.Error(w, "failed to unmarshal request body", http.StatusBadRequest)
		return
	}

	responseBody := map[string]interface{}{
		"message": "Webhook received successfully",
		"data":    requestBody,
	}

	responseData, err := json.Marshal(responseBody)
	if err != nil {
		http.Error(w, "failed to marshal response body", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseData)
}
