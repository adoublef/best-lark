/*
Package sql3 provides an interface for SQLite3 operations.

NOTE, these methods are still needing to be implemented

  - Implement `Find`
  - Implement `Ls`
  - Implement `StatAt`
  - Implement `Rename`
*/
package sql3

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strings"

	"bl.io/gateway/internal/drive"
	"github.com/mattn/go-sqlite3"
	"github.com/rs/xid"
	"go.adoublef.dev/sdk/database/sql3"
	"go.adoublef.dev/sdk/time/julian"
)

type DB struct {
	RWC *sql3.DB
}

func (db *DB) Mkdir(ctx context.Context, dirname string, parent xid.ID) (xid.ID, error) {
	created := xid.New()
	const q = `
	insert into files (id, dir, name, updated_at, v)
	values (?, ?, ?, ?, 0)`
	_, err := db.RWC.Exec(ctx, q, created, ptr(parent), ptr(dirname), julian.Now())
	if err != nil {
		return xid.NilID(), wrap(err)
	}
	return created, nil
}

func (db *DB) Touch(ctx context.Context, filename, mime string, dir xid.ID) (xid.ID, error) {
	created := xid.New()
	const q = `
	insert into files (id, dir, name, ext, mime, updated_at, v)
	values (?, ?, ?, ?, ?, ?, 0)`
	base, ext, _ := strings.Cut(filename, ".")
	_, err := db.RWC.Exec(ctx, q, created, ptr(dir), ptr(base), ptr(ext), ptr(mime), julian.Now())
	if err != nil {
		return xid.NilID(), wrap(err)
	}
	return created, nil
}

func (db *DB) Rename(ctx context.Context, filename string, isDir bool, fid xid.ID, v int64) (err error) {
	const mvDir = `
	update files set
		name = ?
		, updated_at = ?
		, v = v + 1
	where id = ? and v = ?`
	const mv = `
	update files set
		name = ?
		, ext = ?
		, updated_at = ?
		, v = v + 1
	where id = ? and v = ?`
	// .git
	// .gitignore
	var rs sql.Result
	if isDir {
		rs, err = db.RWC.Exec(ctx, mvDir, ptr(filename), julian.Now(), fid, v)
	} else {
		base, ext, _ := strings.Cut(filename, ".")
		rs, err = db.RWC.Exec(ctx, mv, ptr(base), ptr(ext), julian.Now(), fid, v)
	}
	if err != nil {
		return wrap(err)
	}
	if _, err := rowsAffected(rs); err != nil {
		return err
	}
	return nil
}

func (db *DB) Stat(ctx context.Context, fid xid.ID) (drive.FileInfo, int64, error) {
	const q = `
	select
		f.name || coalesce(f.ext, '') as name
		, coalesce(f.mime, 'inode/directory') as mime
		, f.updated_at
		, f.is_dir
		, f.v
	from files f where f.id = ?`
	var found file
	var n int64
	err := db.RWC.QueryRow(ctx, q, fid).Scan(&found.name, &found.mime, &found.t, &found.isDir, &n)
	if err != nil {
		return nil, 0, wrap(err)
	}
	return &found, n, nil
}

func (db *DB) StatAt(ctx context.Context, fid xid.ID, v int64) (drive.FileInfo, error) {
	const q = `
	with cte as (
		select
			f.name
			, f.ext
			, f.mime
			, f.is_dir
			, f.updated_at
			, f.v
			, (1 << 4) - 1 as mask
		from files f where f.id = @id

		union all

		select
			fa.name
			, fa.ext
			, fa.mime
			, fa.is_dir
			, fa.updated_at
			, fa.v
			, fa.mask 
		from files_at fa where fa.id = @id and fa.v >= @version
	)
	select
		f.name
		, f.ext
		, f.mime
		, f.is_dir
		, f.updated_at
		, f.mask
	from cte f order by f.v desc`
	rs, err := db.RWC.Query(ctx, q, sql.Named("id", fid), sql.Named("version", v))
	if err != nil {
		return nil, wrap(err)
	}
	defer rs.Close()
	var filename, extension string
	var found file
	for rs.Next() {
		var name, ext, mime *string
		var isDir *bool
		var t *julian.Time
		var mask int
		err := rs.Scan(&name, &ext, &mime, &isDir, &t, &mask)
		if err != nil {
			return nil, wrap(err)
		}
		if mask&2 != 0 {
			filename = value(name)
		}
		if mask&3 != 0 {
			extension = value(ext)
		}
		if mask&4 != 0 {
			found.mime = value(mime)
		}
		if mask&5 != 0 {
			found.isDir = value(isDir)
		}
		if mask&6 != 0 {
			found.t = value(t)
		}
	}
	if err := rs.Err(); err != nil {
		return nil, wrap(err)
	}
	found.name = strings.Join([]string{filename, extension}, ".")
	return &found, nil
}

//go:embed all:*.up.sql
var embedFS embed.FS

func Up(ctx context.Context, filename string) (*DB, error) {
	rwc, err := sql3.Up(ctx, filename, embedFS)
	if err != nil {
		return nil, fmt.Errorf("drive/sql3: run migrations: %w", err)
	}
	return &DB{rwc}, nil
}

func ptr[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

func value[T any](v *T) T {
	if v == nil {
		var zero T
		return zero
	}
	return *v
}

func rowsAffected(rs sql.Result) (n int64, err error) {
	if n, err = rs.RowsAffected(); err != nil {
		return 0, err
	} else if n == 0 {
		return 0, drive.ErrNotFound
	}
	return
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return errors.Join(err, drive.ErrNotFound)
	}
	// https://github.com/mattn/go-sqlite3/issues/244
	if errors.As(err, new(sqlite3.Error)) {
		switch err.(sqlite3.Error).ExtendedCode {
		case sqlite3.ErrConstraintForeignKey: // maybe this is not treated the same
			return errors.Join(err, drive.ErrConflict)
		case sqlite3.ErrConstraintCheck:
			return errors.Join(err, drive.ErrConflict)
		}
	}
	return fmt.Errorf("drive/sql3: %w", err)
}
