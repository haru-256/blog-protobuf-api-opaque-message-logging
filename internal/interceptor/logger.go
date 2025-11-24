package interceptor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// ReqRespLogger は connect.Interceptor を実装するロギングインターセプタです。
type ReqRespLogger struct {
	logger *slog.Logger
}

// NewReqRespLogger は ReqRespLogger の新しいインスタンスを生成します。
func NewReqRespLogger(logger *slog.Logger) *ReqRespLogger {
	return &ReqRespLogger{
		logger: logger,
	}
}

// NewUnaryInterceptorWithEmptyBody は リクエストとレスポンスのbodyをログ出力する UnaryInterceptorFunc を生成。
// ただし、API_OPAQUE の場合はmessageのFieldが非公開なため、空になってログ出力します。
func (l *ReqRespLogger) NewUnaryInterceptorWithEmptyBody() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			// リクエスト開始ログ
			l.logger.InfoContext(ctx, "request Start",
				slog.String("procedure", req.Spec().Procedure),
				slog.Any("request_body", req.Any()), // API_OPAQUE の場合は空になる
			)
			var code connect.Code
			res, err := next(ctx, req) // 本体処理の実行

			if err != nil {
				code = connect.CodeOf(err)
			} else {
				code = 0 // OK
			}

			// リクエスト終了ログ
			attrs := []slog.Attr{
				slog.String("procedure", req.Spec().Procedure),
				slog.Duration("duration", time.Since(start)),
				slog.String("code", code.String()),
			}
			if err != nil {
				attrs = append(attrs, slog.Any("error", err))
				l.logger.LogAttrs(ctx, slog.LevelError, "request end with error", attrs...)
			} else {
				attrs = append(attrs, slog.Any("response_body", res.Any())) // API_OPAQUE の場合は空になる
				l.logger.LogAttrs(ctx, slog.LevelInfo, "request end", attrs...)
			}
			return res, err
		}
	}
}

func (l *ReqRespLogger) NewUnaryInterceptorWithBody() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			// リクエスト開始ログ
			l.logger.InfoContext(ctx, "request Start",
				slog.String("procedure", req.Spec().Procedure),
				slog.Any("request_body", formatLogValue(req.Any())),
			)
			var code connect.Code
			res, err := next(ctx, req) // 本体処理の実行

			if err != nil {
				code = connect.CodeOf(err)
			} else {
				code = 0 // OK
			}

			// リクエスト終了ログ
			attrs := []slog.Attr{
				slog.String("procedure", req.Spec().Procedure),
				slog.Duration("duration", time.Since(start)),
				slog.String("code", code.String()),
			}
			if err != nil {
				attrs = append(attrs, slog.Any("error", err))
				l.logger.LogAttrs(ctx, slog.LevelError, "request end with error", attrs...)
			} else {
				attrs = append(attrs, slog.Any("response_body", formatLogValue(res.Any())))
				l.logger.LogAttrs(ctx, slog.LevelInfo, "request end", attrs...)
			}
			return res, err
		}
	}
}

// formatLogValue はメッセージをログ出力用に整形します。
// Protocol Buffers メッセージの場合は protojson.Marshal で JSON 文字列化し、
// json.RawMessage として返すことで、ログ内でJSON構造を保持します。
// API_OPAQUE モードでも適切にフィールドが出力されます。
func formatLogValue(msg any) any {
	protoMsg, ok := msg.(proto.Message)
	if !ok {
		return msg
	}

	marshaler := protojson.MarshalOptions{
		Multiline:       false,
		EmitUnpopulated: false,
	}
	jsonBytes, err := marshaler.Marshal(protoMsg)
	if err == nil {
		return json.RawMessage(jsonBytes)
	}

	// Marshal失敗時はfmt.Sprintfにフォールバック
	return fmt.Sprintf("%v", protoMsg)
}
