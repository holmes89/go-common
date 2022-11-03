package api

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/awslabs/aws-lambda-go-api-proxy/gorillamux"
	"github.com/gorilla/mux"
	"go.uber.org/fx"
)

type Routable interface {
	AddRoute(*mux.Router) *mux.Router
}

type Server struct {
	providers []any
	routeType any
}

func (s *Server) RegisterService(i any, t ...any) {
	if len(t) == 1 {
		s.providers = append(s.providers, fx.Annotate(i, fx.As(t)))
		return
	}
	s.providers = append(s.providers, i)
}

func (s *Server) RegisterRouter(i any) {
	s.providers = append(s.providers,
		fx.Annotate(
			i,
			fx.As(s.routeType),
			fx.ResultTags(`group:"routes"`),
		))
}

func (s *Server) RouteAggregator(i any) {
	s.providers = append(s.providers,
		fx.Annotate(
			i,
			fx.ParamTags(`group:"routes"`),
		))
}

func NewServer(routeType any) *Server {
	return &Server{
		routeType: routeType,
	}
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

type Handler interface {
	Handle(context.Context, core.SwitchableAPIGatewayRequest) (*core.SwitchableAPIGatewayResponse, error)
}

type APIGatewayHandler struct {
	adapter *gorillamux.GorillaMuxAdapter
}

func NewAPIGatewayHandler(router *mux.Router) Handler {

	adapter := gorillamux.New(router)
	return &APIGatewayHandler{
		adapter: adapter,
	}
}

func (s *APIGatewayHandler) Handle(ctx context.Context, request core.SwitchableAPIGatewayRequest) (*core.SwitchableAPIGatewayResponse, error) {
	request.Version1().Path = fmt.Sprintf("/v1%s", request.Version1().Path)
	return s.adapter.ProxyWithContext(ctx, request)
}

func function(h Handler) {
	lambda.Start(h.Handle)
}
