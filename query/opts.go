package query

import (
	"context"
	"fmt"
	"strings"
)

type Filter map[string]interface{}
type Sort map[string]string

type Pagination struct {
	Limit         uint64
	Offset        string
	LimitOverride bool
}

type Opts struct {
	Filter
	Sort
	Pagination
}

func ParseOpts(
	ctx context.Context,
	offset string,
	limit int32,
	sort []string,
	filter []string,
) (Opts, error) {
	filters := make(map[string]interface{})
	for _, f := range filter {
		if f == "" {
			continue
		}
		res := strings.Split(f, ":")
		if len(res) != 2 {
			return Opts{}, fmt.Errorf("invalid filter request: %s", f)
		}
		filters[res[0]] = res[1]
	}
	sortParams := make(map[string]string)
	for _, s := range cleanSlice(sort) {
		if s == "" {
			continue
		}
		out := strings.Split(s, ":")
		if len(out) < 2 {
			return Opts{}, fmt.Errorf("invalid sort query: %s", s)
		}
		sortParams[out[0]] = out[1]
	}
	return Opts{
		Filter: filters,
		Sort:   sortParams,
		Pagination: Pagination{
			Limit:         uint64(limit),
			Offset:        offset,
			LimitOverride: limit < 0,
		},
	}, nil
}

// For some reason OpenAPI gen will send a slice with and empty string
func cleanSlice(a []string) []string {
	n := []string{}
	for _, e := range a {
		if e != "" {
			n = append(n, e)
		}
	}
	return n
}
