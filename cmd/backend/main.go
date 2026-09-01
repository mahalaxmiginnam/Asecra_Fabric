package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func okHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("BACKEND: 200 OK")

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "backend: ok")
}

func badRequestHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("BACKEND: 400 Bad Request")

	http.Error(w, "backend: bad request", http.StatusBadRequest)
}

func unauthorizedHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("BACKEND: 401 Unauthorized")

	http.Error(w, "backend: unauthorized", http.StatusUnauthorized)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("BACKEND: 404 Not Found")

	http.Error(w, "backend: not found", http.StatusNotFound)
}

func rateLimitHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("BACKEND: 429 Too Many Requests")

	w.Header().Set("Retry-After", "3")

	http.Error(
		w,
		"backend: rate limited",
		http.StatusTooManyRequests,
	)
}

func serverErrorHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("BACKEND: 500 Internal Server Error")

	http.Error(
		w,
		"backend: internal error",
		http.StatusInternalServerError,
	)
}

func unavailableHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("BACKEND: 503 Service Unavailable")

	http.Error(
		w,
		"backend: service unavailable",
		http.StatusServiceUnavailable,
	)
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("BACKEND: slow request started")

	select {

	case <-time.After(5 * time.Second):
		log.Println("BACKEND: slow request completed")

		fmt.Fprintln(
			w,
			"backend: finished",
		)

	case <-r.Context().Done():

		log.Printf(
			"BACKEND: request cancelled: %v",
			r.Context().Err(),
		)
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf(
		"BACKEND: %s %s",
		r.Method,
		r.URL.Path,
	)

	if r.Method != http.MethodGet {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	w.WriteHeader(http.StatusOK)

	fmt.Fprintln(
		w,
		"hello from backend",
	)
}
func orderHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf(
		"BACKEND: %s %s",
		r.Method,
		r.URL.Path,
	)

	if r.Method != http.MethodPost {
		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	log.Println("BACKEND: creating order")

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	w.WriteHeader(http.StatusCreated)

	fmt.Fprintln(
		w,
		`{"order_id":"order-123","created":true}`,
	)
}
func main() {

	mux := http.NewServeMux()

	mux.HandleFunc("/hello", helloHandler)

	mux.HandleFunc("/ok", okHandler)

	mux.HandleFunc(
		"/bad-request",
		badRequestHandler,
	)

	mux.HandleFunc(
		"/unauthorized",
		unauthorizedHandler,
	)

	mux.HandleFunc(
		"/not-found",
		notFoundHandler,
	)

	mux.HandleFunc(
		"/rate-limit",
		rateLimitHandler,
	)

	mux.HandleFunc(
		"/server-error",
		serverErrorHandler,
	)

	mux.HandleFunc(
		"/unavailable",
		unavailableHandler,
	)

	mux.HandleFunc(
		"/slow",
		slowHandler,
	)
	mux.HandleFunc("/orders", orderHandler)
	server := &http.Server{
		Addr:    ":9000",
		Handler: mux,

		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Println(
		"Asecra Fabric backend listening on :9000",
	)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
