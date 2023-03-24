package api

import (
	"context"

	"github.com/holmes89/go-common/query"
	"github.com/rs/zerolog/log"
)

func NewCrudLogger[T any]() *CRUDLogger[T] {
	return &CRUDLogger[T]{}
}

type CRUDLogger[T any] struct{}

func (l *CRUDLogger[T]) Create(ctx context.Context, t T, opts query.Opts) (T, error) {
	var ty T
	log.Info().Interface("body", t).Type("type", ty).Msg("create called")
	return t, nil
}

func (l *CRUDLogger[T]) Update(ctx context.Context, id string, t T) (T, error) {
	var ty T
	log.Info().Interface("body", t).Type("type", ty).Str("id", id).Msg("update called")
	return t, nil
}
func (l *CRUDLogger[T]) FindAll(ctx context.Context) ([]T, error) {
	var ty T
	log.Info().Type("type", ty).Msg("find all called")
	return nil, nil
}
func (l *CRUDLogger[T]) FindByID(ctx context.Context, id string) (T, error) {
	var t T
	log.Info().Str("id", id).Type("type", t).Msg("find by id called")
	return t, nil
}
func (l *CRUDLogger[T]) Delete(ctx context.Context, id string) error {
	var ty T
	log.Info().Str("id", id).Type("type", ty).Msg("delete called")
	return nil
}
