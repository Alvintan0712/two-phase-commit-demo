package main

import (
	"encoding/json"
	"log"
	"net/http"
)

func CreateOrder(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"message": "Order created",
		"status":  "ok",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(resp)
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/order", CreateOrder)

	log.Println("Starting HTTP server at 127.0.0.1:8000")
	if err := http.ListenAndServe(":8000", mux); err != nil {
		log.Fatal(err)
	}
}
