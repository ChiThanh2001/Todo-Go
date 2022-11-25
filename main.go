package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"todo-app/proto/todogrpc"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type server struct {
	todogrpc.UnimplementedTodoMamagementServer
	DB *gorm.DB
}

type Todo struct {
	gorm.Model
	Name string
}

func (s *server) CreateTodoItem(_ context.Context, req *todogrpc.CreateTodo) (*todogrpc.Todo, error) {
	if req.Name != "" {
		todo := &Todo{
			Name: req.Name,
		}
		s.DB.Create(todo)

		return &todogrpc.Todo{
			Name: todo.Name,
			Id:   int32(todo.ID),
		}, nil
	}

	return nil, status.Error(codes.InvalidArgument, "Todo name is null")
}

func (s *server) GetTodoLists(context.Context, *emptypb.Empty) (*todogrpc.TodoList, error) {
	var todoList []*todogrpc.Todo
	s.DB.Find(&todoList)
	return &todogrpc.TodoList{
		Todos: todoList,
	}, nil
}

func (s *server) GetTodoItemById(_ context.Context, Id *todogrpc.TodoId) (*todogrpc.Todo, error) {
	var todo *todogrpc.Todo
	s.DB.Find(&todo, Id.Id)
	return todo, nil
}

func (s *server) UpdateTodoItem(_ context.Context, req *todogrpc.Todo) (*todogrpc.Todo, error) {
	var todo *todogrpc.Todo
	s.DB.Find(&todo, req.Id)
	if req.Name != "" {
		todo.Name = req.Name
	}
	s.DB.Save(&todo)
	return todo, nil
}

func (s *server) DeleteTodoItem(_ context.Context, Id *todogrpc.TodoId) (*todogrpc.ConfirmMessage, error) {
	var todo *todogrpc.Todo
	result := s.DB.Delete(&todo, Id.Id)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return &todogrpc.ConfirmMessage{
			Message: "not exist todo",
		}, nil
	}

	return &todogrpc.ConfirmMessage{
		Message: "Delete success",
	}, nil
}

func NewServer(db *gorm.DB) *server {
	return &server{
		DB: db,
	}
}

func main() {
	db, err := gorm.Open(mysql.Open("root:2142001@tcp(127.0.0.1:3306)/todo?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&Todo{})

	// Create a listener on TCP port
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalln("Failed to listen:", err)
	}

	// Create a gRPC server object
	s := grpc.NewServer()

	// Attach the Greeter service to the server
	todogrpc.RegisterTodoMamagementServer(s, NewServer(db))
	// Serve gRPC server
	log.Println("Serving gRPC on 0.0.0.0:8080")
	go func() {
		log.Fatalln(s.Serve(lis))
	}()

	// Create a client connection to the gRPC server we just started
	// This is where the gRPC-Gateway proxies the requests
	conn, err := grpc.DialContext(
		context.Background(),
		"0.0.0.0:8080",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalln("Failed to dial server:", err)
	}

	gwmux := runtime.NewServeMux()
	// Register Greeter
	err = todogrpc.RegisterTodoMamagementHandler(context.Background(), gwmux, conn)
	if err != nil {
		log.Fatalln("Failed to register gateway:", err)
	}

	gwServer := &http.Server{
		Addr:    ":8090",
		Handler: gwmux,
	}

	log.Println("Serving gRPC-Gateway on http://0.0.0.0:8090")
	log.Fatalln(gwServer.ListenAndServe())

}
