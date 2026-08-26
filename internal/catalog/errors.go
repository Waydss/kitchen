package catalog

import "errors"

var (
	ErrInvalidID = errors.New("invalid identifier")
	ErrNotFound  = errors.New("restaurant not found")
)
