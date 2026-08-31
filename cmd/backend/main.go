package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("BACKEND: %s %s", r.Method, r.URL.Path)

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "hello from backend")
}

func slowHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("BACKEND: slow request started")

	select {
	case <-time.After(5 * time.Second):
		log.Println("BACKEND: slow request completed")
		fmt.Fprintln(w, "backend finished")

	case <-r.Context().Done():
		log.Printf("BACKEND: request cancelled: %v", r.Context().Err())
	}
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/hello", helloHandler)
	mux.HandleFunc("/slow", slowHandler)

	server := &http.Server{
		Addr:         ":9000",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	log.Println("Asecra Fabric backend listening on :9000")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
