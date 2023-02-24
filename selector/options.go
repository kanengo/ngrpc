package selector

import "context"

type SelectOptions struct {
	NodeFilters []Filter[Node]
}

type Filter[T any] func(context.Context, []T) []T

type SelectOption func(options *SelectOptions)
