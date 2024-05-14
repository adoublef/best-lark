package sql3

import (
	"time"

	"github.com/rs/xid"
	"go.adoublef.dev/sdk/time/julian"
)

type file struct {
	id    xid.ID
	name  string
	mime  string
	isDir bool
	t     julian.Time
}

func (f file) ID() xid.ID { return f.id }

func (f file) Name() string { return f.name }

func (f file) Mime() string {
	if f.mime == "" {
		return "application/octet-stream"
	}
	return f.mime
}

func (f file) IsDir() bool { return f.isDir }

func (f file) Time(loc *time.Location) time.Time {
	if loc == nil {
		return f.t.Time()
	}
	return f.t.Time().In(loc)
}
