package drive

import "errors"

var (
	ErrNotFound = errors.New("drive: not found")
	ErrConflict = errors.New("drive: conflict")
)
