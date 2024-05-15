package unix

import "errors"

var (
	ErrNotFound = errors.New("unix: not found")
	ErrConflict = errors.New("unix: conflict")
)
