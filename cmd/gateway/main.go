package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const backendTimeout = 2 * time.Second

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ok")
}

func withTimeout(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func main() {
	backendURL, err := url.Parse("http://localhost:9000")
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api")

		if req.URL.Path == "" {
			req.URL.Path = "/"
		}

		log.Printf(
			"GATEWAY: forwarding %s %s",
			req.Method,
			req.URL.Path,
		)
	}

	proxy.ErrorHandler = func(
		w http.ResponseWriter,
		r *http.Request,
		err error,
	) {
		log.Printf("GATEWAY: proxy error: %v", err)

		if r.Context().Err() != nil {
			http.Error(
				w,
				"upstream request cancelled",
				http.StatusGatewayTimeout,
			)
			return
		}

		http.Error(
			w,
			"bad gateway",
			http.StatusBadGateway,
		)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)

	mux.Handle(
		"/api/",
		withTimeout(backendTimeout, proxy),
	)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Println("Asecra Fabric gateway listening on :8080")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}