// Package main defines a mock server for benchmarking.
package main

import (
	"bytes"
	"io"
	"log"
	"log/slog"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/mock", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
		case http.MethodPost:
			w.WriteHeader(http.StatusOK)

			_, err := io.Copy(w, r.Body)
			if err != nil {
				slog.Error(err.Error())
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			body := io.NopCloser(bytes.NewBufferString(`{"data":{"users":[{"id":1}]}}`))

			w.WriteHeader(http.StatusOK)

			_, err := io.Copy(w, body)
			if err != nil {
				slog.Error(err.Error())
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	err := http.ListenAndServe(":8080", mux) //nolint:gosec
	if err != nil {
		log.Fatal(err)
	}
}
