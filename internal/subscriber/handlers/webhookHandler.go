package handlers

import "net/http"

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	// Parse request body.
	// Validate request body integrity.
	// Store it to s3
	// Be careful when reading from r.Body. It's a stream, not a buffer. So once you read it it will be drained from the connection. Solution is: Either store it on context and read from the cache later, or restore it to io stream after reading.
	w.WriteHeader(200)
	w.Write([]byte(`
		{
			"message": "Recieved request"
		}
	`))

}
