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
	"log"
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
	log.Printf("%q, %q", base, ext)
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
		, v = v + 1
	where id = ? and v = ?`
	const mv = `
	update files set
		name = ?
		, ext = ?
		, v = v + 1
	where id = ? and v = ?`
	// .git
	// .gitignore
	var rs sql.Result
	if isDir {
		rs, err = db.RWC.Exec(ctx, mvDir, ptr(filename), fid, v)
	} else {
		base, ext, _ := strings.Cut(filename, ".")
		log.Printf("%q, %q", base, ext)
		rs, err = db.RWC.Exec(ctx, mv, ptr(base), ptr(ext), fid, v)
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

		union

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
	var filename, extension, contentType string
	var updatedAt julian.Time
	var isDir bool
	for rs.Next() {
		var name, ext, mime *string
		var is *bool
		var mask int
		err := rs.Scan(&name, &ext, &mime, &is, &updatedAt, &mask)
		if err != nil {
			return nil, wrap(err)
		}
		log.Printf("name=%v, ext=%v, mask=%d", name, ext, mask)
		// name 1 mask = 3 or 1
		if mask&2 != 0 {
			filename = *name
		}
		// ext 3
		if mask&3 != 0 {
			extension = *ext
		}
		// mime 3
		// if mask&4 != 0 {
		// 	contentType = *mime
		// }
		// is_dir 4
		// if mask&5 != 0 {
		// 	isDir = *is
		// }
		// updated_at
	}
	if err := rs.Err(); err != nil {
		return nil, wrap(err)
	}
	filename = strings.Join([]string{filename, extension}, ".")
	return &file{fid, filename, contentType, isDir, updatedAt}, nil
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
