package main

import (
	"log"
	"net/http"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/Alvintan0712/two-phase-commit-demo/shared/api/proto"
	"github.com/Alvintan0712/two-phase-commit-demo/shared/pkg/rest"
)

var (
	coordinatorServiceClient pb.CoordinatorServiceClient
	userServiceClient        pb.UserServiceClient
	orderServiceClient       pb.OrderServiceClient
)

// can try user id 04937668-e73f-4035-a7d7-8f8db1a679e8
func CreateOrder(w http.ResponseWriter, r *http.Request) {
	var orderReq *pb.PlaceOrderRequest
	if err := rest.ReadJSON(r, &orderReq); err != nil {
		rest.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := coordinatorServiceClient.PlaceOrder(r.Context(), orderReq)
	if err != nil {
		rest.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if !resp.Success {
		rest.WriteError(w, http.StatusBadRequest, resp.Message)
		return
	}

	rest.WriteJSON(w, http.StatusOK, map[string]string{
		"message": "Order created",
		"status":  "success",
	})
}

func GetUser(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("id")
	if userId == "" {
		rest.WriteError(w, http.StatusBadRequest, "missing user id")
		return
	}

	user, err := userServiceClient.GetUser(r.Context(), &pb.GetUserRequest{UserId: userId})
	if err != nil {
		rest.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rest.WriteJSON(w, http.StatusOK, user)
}

func GetOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := orderServiceClient.GetOrders(r.Context(), nil)
	if err != nil {
		rest.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rest.WriteJSON(w, http.StatusOK, orders)
}

func main() {
	coordinatorServiceAddr, ok := syscall.Getenv("COORDINATOR_SERVICE")
	if !ok {
		coordinatorServiceAddr = "127.0.0.1:8082"
	}

	userServiceAddr, ok := syscall.Getenv("USER_SERVICE")
	if !ok {
		userServiceAddr = "127.0.0.1:8080"
	}

	orderServiceAddr, ok := syscall.Getenv("ORDER_SERVICE")
	if !ok {
		orderServiceAddr = "127.0.0.1:8081"
	}

	mux := http.NewServeMux()

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	coordinatorServiceClientConn, err := grpc.NewClient(coordinatorServiceAddr, opts...)
	if err != nil {
		log.Fatal(err)
	}
	userServiceClientConn, err := grpc.NewClient(userServiceAddr, opts...)
	if err != nil {
		log.Fatal(err)
	}
	orderServiceClientConn, err := grpc.NewClient(orderServiceAddr, opts...)
	if err != nil {
		log.Fatal(err)
	}

	coordinatorServiceClient = pb.NewCoordinatorServiceClient(coordinatorServiceClientConn)
	userServiceClient = pb.NewUserServiceClient(userServiceClientConn)
	orderServiceClient = pb.NewOrderServiceClient(orderServiceClientConn)

	mux.HandleFunc("POST /v1/order", CreateOrder)
	mux.HandleFunc("GET /v1/user/{id}", GetUser)
	mux.HandleFunc("GET /v1/order", GetOrders)

	log.Println("Starting HTTP server at 127.0.0.1:8000")
	if err := http.ListenAndServe(":8000", mux); err != nil {
		log.Fatal(err)
	}
}
