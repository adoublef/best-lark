package unix

import (
	"time"

	"github.com/rs/xid"
)

type FileInfo interface {
	ID() xid.ID
	Name() string
	Mime() string
	IsDir() bool
	Time(loc *time.Location) time.Time
}
