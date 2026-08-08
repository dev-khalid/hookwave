package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type contextKey string

const requestBodyContextKey contextKey = "requestBody"

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(requestDataValidationMiddleware)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {

		response, _ := json.MarshalIndent(map[string]string{
			"message": "Server is healthy and running!",
		}, "", "  ")

		w.Header().Set("Content-Type", "application/json")
		w.Write(response)
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		errorResponse := map[string]interface{}{
			"error": "Route not found",
			"path":  r.URL.Path,
		}
		response, _ := json.MarshalIndent(errorResponse, "", "  ")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write(response)
	})

	v1Api := chi.NewRouter()
	v1Api.Mount("/api/v1", r)
	log.Println("Server running at port: 8000 🚀")
	log.Fatal(http.ListenAndServe(":8000", v1Api))
}

func requestDataValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			defer r.Body.Close()
			var requestBody map[string]interface{}

			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "Invalid or missing JSON body",
				})
				return
			}

			if requestBody["name"] == nil || requestBody["email"] == nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "Missing required fields: name and email",
				})
				return
			}

			r = r.WithContext(context.WithValue(r.Context(), requestBodyContextKey, requestBody))
		}

		if r.Method == http.MethodGet {
			log.Println("No validation needed!")
		}

		next.ServeHTTP(w, r)
	})
}
