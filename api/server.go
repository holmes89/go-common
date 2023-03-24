package api

import (
	"context"
	"fmt"
	"net/http"

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
	s.providers = append(s.providers, i)
}

func AsComponent[T any](f any, paramTags string, resultTags string) any {
	annotations := []fx.Annotation{}
	if resultTags != "" {
		annotations = append(annotations, fx.ResultTags(resultTags))
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

func (s *APIGatewayHandler) Handle(ctx context.Context, request core.SwitchableAPIGatewayRequest) (*core.SwitchableAPIGatewayResponse, error) {
	fmt.Println(request.Version1().Path)
	return s.adapter.ProxyWithContext(ctx, request)
}

func function(h LambdaHandler) {
	lambda.Start(h.Handle)
}
