package main

import (
	"encoding/json"
	"log"
	"net/http"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Alvintan0712/two-phase-commit-demo/shared/api/proto"
)

var (
	coordinatorServiceClient pb.CoordinatorServiceClient
)

func CreateOrder(w http.ResponseWriter, r *http.Request) {
	orderReq := &pb.PlaceOrderRequest{
		UserId: "04937668-e73f-4035-a7d7-8f8db1a679e8",
		Price:  100,
	}
	_, err := coordinatorServiceClient.PlaceOrder(r.Context(), orderReq)
	if err != nil {
		resp := map[string]string{
			"message": err.Error(),
			"status":  "error",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := map[string]string{
		"message": "Order created",
		"status":  "success",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(resp)
}

func main() {
	coordinatorServiceAddr, ok := syscall.Getenv("COORDINATOR_SERVICE")
	if !ok {
		coordinatorServiceAddr = "127.0.0.1:8082"
	}

	mux := http.NewServeMux()

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	coordinatorServiceClientConn, err := grpc.NewClient(coordinatorServiceAddr, opts...)
	if err != nil {
		log.Fatal(err)
	}

	coordinatorServiceClient = pb.NewCoordinatorServiceClient(coordinatorServiceClientConn)

	mux.HandleFunc("POST /v1/order", CreateOrder)

	log.Println("Starting HTTP server at 127.0.0.1:8000")
	if err := http.ListenAndServe(":8000", mux); err != nil {
		log.Fatal(err)
	}
}
