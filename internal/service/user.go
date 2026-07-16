package service

import (
	"context"
	"auth-service/api/proto"

	"go.opentelemetry.io/otel"
)

// UserServer реализует интерфейс proto.UserServiceServer
type UserServer struct {
	// В новых версиях gRPC-Go обязательно встраивать заглушку
	// для обратной совместимости, если добавится новый метод в .proto
	proto.UnimplementedUserServiceServer

	// Здесь могут быть твои репозитории БД, кэш и т.д.
}

var grpcTracer = otel.Tracer("replicator-service")


// Пишем сам метод, который мы описали в .proto
func (s *UserServer) GetUser(ctx context.Context, req *proto.GetUserRequest) (*proto.GetUserResponse, error) {
	// Достаем ID из запроса (gRPC сам всё распаковал)
	userID := req.GetId()

	// ... тут поход в БД ...

	// Формируем ответ
	return &proto.GetUserResponse{
		Id:    userID,
		Name:  "Иван Иванов",
		Email: "ivan@example.com",
	}, nil
}
