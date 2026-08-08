/**
1. Create a simple server that listens on port 8080 and responds with "Hello, World!" to any HTTP GET request.
2. Use the net/http package to handle incoming requests.
3. Create a logger middleware that logs the request method, URL, Time taken, Response status code for each incoming request.
4. Create another middleware that adds a custom header "X-Custom-Header: MyValue" to each response.
5. Middleware chaining.
6. Subrouting. e.g, stamp a global path
7.
*/

package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type contextKey string

const requestBodyContextKey contextKey = "requestBody"

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// We are overriding the WriteHeader method of the http.ResponseWriter interface to capture/set extra properties...
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func main() {
	http.HandleFunc("POST /api/{id}/user", loggerMiddleware(requestDataValidationMiddleware(func(res http.ResponseWriter, req *http.Request) {

		requestBody, _ := req.Context().Value(requestBodyContextKey).(map[string]interface{})

		requestMetadata := map[string]interface{}{
			"host":      req.Host,
			"url":       req.URL,
			"method":    req.Method,
			"body":      requestBody,
			"pathParam": req.PathValue("id"),
		}
		requestData, err := json.Marshal(requestMetadata)
		if err != nil {
			log.Println("Failed to marshal response:", err)
			res.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Println("Request data: ", string(requestData))
		res.WriteHeader(202)
		res.Write(requestData)
	})))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Route not found",
			"path":  r.URL.Path,
		})
	})

	log.Println("Server running at port: 8080 🚀")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func loggerMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestMetadata := map[string]interface{}{
			"method":  r.Method,
			"url":     r.URL.Path,
			"host":    r.Host,
			"headers": r.Header,
		}

		data, _ := json.MarshalIndent(requestMetadata, "", "	")

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()
		log.Println("Request metadata: ", string(data))
		next.ServeHTTP(wrapped, r)
		log.Println("Time Elapsed: ", time.Since(start))
		log.Println("Response Status Code: ", wrapped.statusCode)
	}
}

func requestDataValidationMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

		next.ServeHTTP(w, r)
	})
}

// Need to go through the Chi router doc.
