package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"go.uber.org/fx"
)

type Route interface {
	http.Handler
	Pattern() string
}

type Server struct {
	providers []any
}

func (s *Server) Register(i any) {
	log.Info().Interface("service", i).Msg("registering service...")
	s.providers = append(s.providers, i)
}

func (s *Server) RegisterAll(i []any) {
	for _, i := range i {
		s.Register(i)
	}
}

func AsComponent[T any](f any, paramTags string, resultTags string) any {
	annotations := []fx.Annotation{}
	if resultTags != "" {
		for _, t := range strings.Split(resultTags, ",") {
			annotations = append(annotations, fx.ResultTags(t))
		}
	}
	if paramTags != "" {
		annotations = append(annotations, fx.ParamTags(paramTags))
	}
	annotations = append(annotations, fx.As(new(T)))
	log.Info().Str("result_tags", resultTags).Str("param_tags", paramTags).Type("type", f).Type("as", new(T)).Msg("registering service...")
	return fx.Annotate(
		f,
		annotations...,
	)
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run() {
	handler := fx.Annotate(NewAPIGatewayHandler)
	s.providers = append(s.providers, handler)
	fx.New(
		fx.Provide(
			s.providers...,
		),
		fx.Invoke(function),
	).Run()
}

type LambdaHandler interface {
	Handle(context.Context, core.SwitchableAPIGatewayRequest) (*core.SwitchableAPIGatewayResponse, error)
}

type APIGatewayHandler struct {
	adapter *gorillamux.GorillaMuxAdapter
}

func NewAPIGatewayHandler(router *mux.Router) LambdaHandler {

	adapter := gorillamux.New(router)
	return &APIGatewayHandler{
		adapter: adapter,
	}
}

type contextKeys int

const (
	userUIDKey contextKeys = iota
)

// CtxWithUserUID will return a context with UID stored as value.
func CtxWithUserUID(ctx context.Context, uid interface{}) context.Context {
	return context.WithValue(ctx, userUIDKey, uid)
}

// UserUIDFromCtx will return user uid stored in context.
func UserUIDFromCtx(ctx context.Context) string {
	s, ok := ctx.Value(userUIDKey).(string)
	if !ok {
		return ""
	}
	return s
}

func (s *APIGatewayHandler) Handle(ctx context.Context, request core.SwitchableAPIGatewayRequest) (*core.SwitchableAPIGatewayResponse, error) {
	uctx := CtxWithUserUID(ctx, request.Version1().RequestContext.Identity.AccountID)
	if devID := os.Getenv("DEV_ID"); devID != "" {
		uctx = CtxWithUserUID(ctx, devID)
		fmt.Println(request.Version1().Path)
	}
	if debug := os.Getenv("DEBUG"); debug != "" {
		fmt.Println(request.Version1().Path)
	}
	return s.adapter.ProxyWithContext(uctx, request)
}

func function(h LambdaHandler) {
	lambda.Start(h.Handle)
}
