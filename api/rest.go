package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/holmes89/go-common/query"
	"github.com/rs/zerolog/log"
)

type RESTApi struct {
}

type Handler interface {
	Mount() (string, string, http.HandlerFunc)
}

type Handle[T any] struct {
	name        string
	Path        string
	RequestType string
	Handle      http.HandlerFunc
}

func (n *Handle[T]) Name() string {
	return n.name
}

func NewHandler[T any](name string, path string, requestType string, fun http.HandlerFunc) *Handle[T] {
	return &Handle[T]{
		name:        name,
		Path:        path,
		RequestType: requestType,
		Handle:      fun,
	}
}

func NewGetHandler[T any](name string, path string, fun http.HandlerFunc) *Handle[T] {
	return &Handle[T]{
		name:        name,
		Path:        path,
		RequestType: "GET",
		Handle:      fun,
	}
}

func NewPostHandler[T any](name string, path string, fun http.HandlerFunc) *Handle[T] {
	return &Handle[T]{
		name:        name,
		Path:        path,
		RequestType: "POST",
		Handle:      fun,
	}
}

func NewFindByIDHandler[T any](name string, path string, repo Repository[T]) *Handle[T] {
	path = filepath.Join(path, "{id}")
	return &Handle[T]{
		name:        name,
		Path:        path,
		RequestType: "GET",
		Handle: func(w http.ResponseWriter, r *http.Request) {
			vars := mux.Vars(r)
			ctx := r.Context()
			id := vars["id"]

			resource, err := repo.FindByID(ctx, id)
			if err != nil {
				http.Error(w, "unable to find resource", http.StatusInternalServerError)
				return
			}

			EncodeJSONResponse(r.Context(), w, resource)
		},
	}
}

func NewFindAllHandler[T any](name string, path string, repo Repository[T]) *Handle[T] {
	return &Handle[T]{
		name:        name,
		Path:        path,
		RequestType: "GET",
		Handle: func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			// query.ParseOpts()
			resource, err := repo.FindAll(ctx, query.Opts{}) // todo redo parser
			if err != nil {
				http.Error(w, "unable to find resource", http.StatusInternalServerError)
				return
			}
			EncodeJSONResponse(r.Context(), w, resource)
		},
	}
}

func NewCreateHandler[T any](name string, path string, factory Factory[T]) *Handle[T] {
	return &Handle[T]{
		name:        name,
		Path:        path,
		RequestType: "POST",
		Handle: func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			resource, err := extractBody[T](r)
			if err != nil {
				http.Error(w, "invalid resource", http.StatusBadRequest)
				return
			}

			resource, err = factory.Create(ctx, resource)
			if err != nil {
				http.Error(w, "unable to create resource", http.StatusInternalServerError)
				return
			}
			EncodeJSONResponse(r.Context(), w, resource)
		},
	}
}

func extractBody[T any](r *http.Request) (resource T, err error) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("unable to read body")
		return resource, errors.New("unable to read body")
	}
	defer r.Body.Close()

	if err := json.Unmarshal(b, &resource); err != nil {
		log.Error().Err(err).Msg("unable to unmarshal body")
		return resource, errors.New("unable to unmarshal body")
	}
	return resource, nil
}

func (h *Handle[T]) Mount() (string, string, http.HandlerFunc) {
	return h.Path, h.RequestType, h.Handle
}

type Control struct {
	name     string
	RootPath string
	Handlers []Handler
}

type Controller interface {
	Mount(*mux.Router)
}

func (n *Control) Name() string {
	return n.name
}

func (n *Control) Mount(mr *mux.Router) {
	log.Info().Str("path", n.RootPath).Msg("creating controller...")
	r := mr.PathPrefix(fmt.Sprintf("/%s", n.RootPath)).Subrouter()

	for _, handler := range n.Handlers {
		path, t, fun := handler.Mount()
		log.Info().Str("path", path).Str("root", n.RootPath).Msg("mounting path")
		r.HandleFunc(fmt.Sprintf("/%s", path), fun).
			Methods(t, "OPTIONS")
	}
}

func NewCRUDController[T any](name string, path string, svc CRUD[T]) Controller {
	return NewController(path, []Handler{
		NewCreateHandler[T](fmt.Sprintf("createHandler%s", name), "", svc),
		NewFindByIDHandler[T](fmt.Sprintf("findbyIDHandler%s", name), "", svc),
		NewFindAllHandler[T](fmt.Sprintf("findAllHandler%s", name), "", svc),
	})
}

type Router struct {
	name        string
	Controllers []Controller
}

func NewController(root string, handlers []Handler) Controller {
	//todo check for no router, create default

	return &Control{
		RootPath: root,
		Handlers: handlers,
	}
}

func NewRouter(controllers []Controller) *mux.Router {
	//todo check for no router, create default
	log.Info().Int("count", len(controllers)).Msg("creating controllers...")
	mux := mux.NewRouter()
	for _, c := range controllers {
		c.Mount(mux)
	}
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Authorization"})
	originsOk := handlers.AllowedOrigins([]string{"*"}) // TODO env
	methodsOk := handlers.AllowedMethods([]string{"GET", "HEAD", "POST", "PUT", "PATCH", "OPTIONS", "DELETE"})

	cors := handlers.CORS(originsOk, headersOk, methodsOk)
	mux.Use(cors)
	return mux
}

type CRUD[T any] interface {
	Factory[T]
	Repository[T]
	Removal[T]
}

type Factory[T any] interface {
	Create(context.Context, T) (T, error)
	Update(context.Context, string, T) (T, error)
}

type Repository[T any] interface {
	FindAll(context.Context, query.Opts) ([]T, error)
	FindByID(context.Context, string) (T, error)
}

type Removal[T any] interface {
	Delete(context.Context, string) error
}

// EncodeJSONResponse will take a given interface and encode the value as JSON
func EncodeJSONResponse[T any](ctx context.Context, w http.ResponseWriter, response T) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return enc.Encode(response)
}
