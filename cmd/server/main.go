package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	myservice "github.com/haru-256/blog-protobuf-api-opaque-message-logging/gen/go/myservice/v1"
	myserviceconnect "github.com/haru-256/blog-protobuf-api-opaque-message-logging/gen/go/myservice/v1/myservicev1connect"
	"github.com/haru-256/blog-protobuf-api-opaque-message-logging/internal/interceptor"
)

// myServiceImpl は MyService の実装です。
type myServiceImpl struct {
	logger *slog.Logger
	myserviceconnect.UnimplementedMyServiceHandler
}

func NewMyServiceImpl(logger *slog.Logger) *myServiceImpl {
	return &myServiceImpl{
		logger: logger,
	}
}

// GetUser (Unary RPC) の実装
func (s *myServiceImpl) GetUser(
	ctx context.Context,
	req *connect.Request[myservice.GetUserRequest],
) (*connect.Response[myservice.GetUserResponse], error) {
	s.logger.InfoContext(ctx, "--- [Server Logic] GetUser called ---")
	// ダミーのロジック
	userId := req.Msg.GetUserId()
	if userId == "error" {
		s.logger.ErrorContext(ctx, "User not found", "userId", userId)
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}

	user := &myservice.User{}
	user.SetUserId(userId)
	user.SetName("haru256")
	resp := &myservice.GetUserResponse{}
	resp.SetUser(user)

	return connect.NewResponse(resp), nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// args
	parsed := flag.Bool("parsed", false, "Use parsed message type to log API_OPAQUE messages")
	flag.Parse()

	// ロガーの準備 (DEBUGレベルでペイロードも出力)
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: false}),
	)
	// インターセプタのインスタンス化
	reqRespLogger := interceptor.NewReqRespLogger(logger)
	var interceptor connect.UnaryInterceptorFunc
	logger.InfoContext(ctx, "API_OPAQUE message logging configuration", "parsed", *parsed)
	if *parsed {
		logger.InfoContext(ctx, "Using parsed body interceptor for API_OPAQUE messages")
		interceptor = reqRespLogger.NewUnaryInterceptorWithBody()
	} else {
		logger.InfoContext(ctx, "Using empty body interceptor for API_OPAQUE messages")
		interceptor = reqRespLogger.NewUnaryInterceptorWithEmptyBody()
	}
	// ハンドラの初期化時にオプションとして渡す
	mux := http.NewServeMux()
	path, handler := myserviceconnect.NewMyServiceHandler(
		NewMyServiceImpl(logger),
		connect.WithInterceptors(interceptor),
	)
	mux.Handle(path, handler)

	// reflection
	reflector := grpcreflect.NewStaticReflector(
		myserviceconnect.MyServiceName,
	)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

	// サーバーの起動
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8081"
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		logger.ErrorContext(ctx, "Failed to start server", "error", err)
		return
	}
	srv := &http.Server{
		Addr:    ln.Addr().String(),
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}
	go func() {
		logger.InfoContext(ctx, "Server started", "address", ln.Addr().String())
		if srvErr := srv.Serve(ln); srvErr != nil && !errors.Is(srvErr, http.ErrServerClosed) {
			logger.ErrorContext(ctx, "Server failed", "error", srvErr)
			return
		}
	}()
	<-ctx.Done()

	// サーバーをグレースフルシャットダウン
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = srv.Shutdown(shutdownCtx); err != nil {
		logger.ErrorContext(shutdownCtx, "Server shutdown failed", "error", err)
		return
	} else {
		logger.InfoContext(shutdownCtx, "Server stopped gracefully")
	}
}
