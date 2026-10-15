package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	fmt.Fprintf(os.Stderr, "Starting minimal HTTP server on :8080\n")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(os.Stderr, "Request received: %s %s\n", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Hello World from Portfolio Terminal!\n")
		fmt.Fprintf(w, "Go dependencies not resolved, but basic server works.\n")
	})

	server := &http.Server{
		Addr: "0.0.0.0:8080",
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
	}
}
