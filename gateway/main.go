package main

import (
	"encoding/json"
	"log"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/two-phase-commit-demo/gateway/proto"
)

const (
	userServiceAddr  = "127.0.0.1:8080"
	orderServiceAddr = "127.0.0.1:8081"
)

var (
	userServiceClient  pb.UserServiceClient
	orderServiceClient pb.OrderServiceClient
)

func CreateOrder(w http.ResponseWriter, r *http.Request) {
	deductReq := &pb.DeductWalletRequest{
		OrderId: "id",
		Price:   100,
	}
	_, err := userServiceClient.DeductWallet(r.Context(), deductReq)
	if err != nil {
		resp := map[string]string{
			"message": err.Error(),
			"status":  "error",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(resp)
	}

	orderReq := &pb.CreateOrderRequest{
		OrderId: "id",
		Price:   100,
	}
	_, err = orderServiceClient.CreateOrder(r.Context(), orderReq)
	if err != nil {
		resp := map[string]string{
			"message": err.Error(),
			"status":  "error",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(resp)
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
	mux := http.NewServeMux()

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	userServiceClientConn, err := grpc.NewClient(userServiceAddr, opts...)
	if err != nil {
		log.Fatal(err)
	}

	orderServiceClientConn, err := grpc.NewClient(orderServiceAddr, opts...)
	if err != nil {
		log.Fatal(err)
	}

	userServiceClient = pb.NewUserServiceClient(userServiceClientConn)
	orderServiceClient = pb.NewOrderServiceClient(orderServiceClientConn)

	mux.HandleFunc("POST /v1/order", CreateOrder)

	log.Println("Starting HTTP server at 127.0.0.1:8000")
	if err := http.ListenAndServe(":8000", mux); err != nil {
		log.Fatal(err)
	}
}
