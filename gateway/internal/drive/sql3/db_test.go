/*
| raw        | dir (name) | base (name) | ext        | mime
| src        | src        | null        | null       | null
| .git       | .git       | null        | null       | null
| .gitignore | null       | null        | .gitignore | test/plain
| README.md  | null       | README      | md         | text/plain
| ffmpeg     | null       | ffmpeg      | null       | application/octant-stream
*/
package sql3_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"bl.io/gateway/internal/drive"
	. "bl.io/gateway/internal/drive/sql3"
	"github.com/rs/xid"
	"go.adoublef.dev/is"
)

func Test_DB_Mkdir(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		for _, tc := range []string{"src", ".git"} {
			t.Run(tc, run(func(t *testing.T, db *DB) {
				is := is.NewRelaxed(t)
				_, err := db.Mkdir(context.TODO(), tc, xid.NilID())
				is.NoErr(err)
			}))
		}
	})

	t.Run("Root", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)

		_, err := db.Mkdir(context.TODO(), "src", xid.NilID())
		is.NoErr(err)

		_, err = db.Mkdir(context.TODO(), "inc", xid.NilID())
		is.NoErr(err)
	}))

	t.Run("NoName", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		_, err := db.Mkdir(context.TODO(), "", xid.NilID())
		is.Err(err, drive.ErrConflict)
	}))

	t.Run("NoParent", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		_, err := db.Mkdir(context.TODO(), "cmd", xid.New())
		is.Err(err, drive.ErrConflict)
	}))

	// Nested
	t.Run("Nested", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		// inputs
		var (
			src = "src"
			cmd = "cmd"
		)

		parent, err := db.Mkdir(context.TODO(), src, xid.NilID())
		is.NoErr(err)

		_, err = db.Mkdir(context.TODO(), cmd, parent)
		is.NoErr(err)
	}))
}

func Test_DB_Touch(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		for _, tc := range []string{"README.md", ".gitignore", "ffmpeg"} {
			t.Run(tc, run(func(t *testing.T, db *DB) {
				is := is.NewRelaxed(t)
				// inputs
				var (
					mime = "application/octet-stream"
				)
				_, err := db.Touch(context.TODO(), tc, mime, xid.NilID())
				is.NoErr(err)
			}))
		}
	})

	t.Run("NoName", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		// inputs
		var (
			mime = "application/octet-stream"
		)
		_, err := db.Touch(context.TODO(), "", mime, xid.NilID())
		is.Err(err, drive.ErrConflict)
	}))

	t.Run("Root", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		// inputs
		var (
			mime = "application/octet-stream"
		)

		_, err := db.Touch(context.TODO(), "README.md", mime, xid.NilID())
		is.NoErr(err)

		_, err = db.Touch(context.TODO(), ".gitignore", mime, xid.NilID())
		is.NoErr(err)
	}))

	t.Run("NoMIME", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		_, err := db.Touch(context.TODO(), "README.md", "", xid.NilID())
		is.Err(err, drive.ErrConflict)
	}))

	t.Run("NoParent", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		_, err := db.Touch(context.TODO(), "main.go", "text/plain", xid.New())
		is.Err(err, drive.ErrConflict)
	}))

	t.Run("Nested", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		// inputs
		var (
			src  = "src"
			main = "main.go"
		)

		parent, err := db.Mkdir(context.TODO(), src, xid.NilID())
		is.NoErr(err)

		_, err = db.Mkdir(context.TODO(), main, parent)
		is.NoErr(err)
	}))
}

func Test_DB_Stat(t *testing.T) {
	t.Run("OK", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		var (
			filename = "main.go"
			mime     = "text/plain"
		)
		created, err := db.Touch(context.TODO(), filename, mime, xid.NilID())
		is.NoErr(err)

		info, _, err := db.Stat(context.TODO(), created)
		is.NoErr(err)

		is.Equal(info.IsDir(), false)
	}))

	t.Run("IsDir", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		var (
			dirname = "src"
		)
		created, err := db.Mkdir(context.TODO(), dirname, xid.NilID())
		is.NoErr(err)

		info, _, err := db.Stat(context.TODO(), created)
		is.NoErr(err)

		is.Equal(info.IsDir(), true)
	}))
}

func Test_DB_Rename(t *testing.T) {
	t.Run("File", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		var (
			filename = "main_test.go"
			renamed  = "main.go"
			mime     = "text/plain"
		)
		created, err := db.Touch(context.TODO(), filename, mime, xid.NilID())
		is.NoErr(err) // version 0

		err = db.Rename(context.TODO(), renamed, false, created, 0)
		is.NoErr(err)
	}))

	t.Run("Dir", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		var (
			dirname = "bin"
			renamed = "dist"
		)
		created, err := db.Mkdir(context.TODO(), dirname, xid.NilID())
		is.NoErr(err) // version 0

		err = db.Rename(context.TODO(), renamed, true, created, 0)
		is.NoErr(err)
	}))

	t.Run("Conflict", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		var (
			dirname = "bin"
		)
		created, err := db.Mkdir(context.TODO(), dirname, xid.NilID())
		is.NoErr(err) // version 0

		err = db.Rename(context.TODO(), "", true, created, 0)
		is.Err(err, drive.ErrConflict)
	}))
}

func Test_DB_StatAt(t *testing.T) {
	t.Run("First", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		var (
			filename = "main.go"
			mime     = "text/plain"
		)
		created, err := db.Touch(context.TODO(), filename, mime, xid.NilID())
		is.NoErr(err) // version 0

		info, err := db.StatAt(context.TODO(), created, 0)
		is.NoErr(err)

		is.Equal(info.Name(), filename)
	}))

	t.Run("File", func(t *testing.T) {
		type testcase struct {
			init, rename string
		}

		for name, tc := range map[string]testcase{
			"Ext":    {init: "Dockerfile.dev", rename: "Dockerfile"},
			"NoExt":  {init: "Dockerfile", rename: "Dockerfile.dev"},
			"NoName": {init: ".dockerignore", rename: "prod.dockerignore"},
			"Name":   {init: "prod.dockerignore", rename: ".dockerignore"},
			"Both":   {init: "main.ts", rename: "index.js"},
		} {
			t.Run(name, run(func(t *testing.T, db *DB) {
				is := is.NewRelaxed(t)

				created, err := db.Touch(context.TODO(), tc.init, "text/plain", xid.NilID())
				is.NoErr(err) // version 0

				// time.Sleep(time.Second * 1)

				err = db.Rename(context.TODO(), tc.rename, false, created, 0)
				is.NoErr(err)

				info, err := db.StatAt(context.TODO(), created, 0)
				is.NoErr(err)

				// t.Logf("%q, %q", tc.init, tc.rename)
				is.Equal(info.Name(), tc.init)
			}))
		}
	})
}

func Test_DB_Mv(t *testing.T) {
	t.Run("OK", run(func(t *testing.T, db *DB) {
		is := is.NewRelaxed(t)
		var (
			filename = "main.go"
			mime     = "text/plain"
			dirname  = "src"
		)
		dir, err := db.Mkdir(context.TODO(), dirname, xid.NilID())
		is.NoErr(err)

		file, err := db.Touch(context.TODO(), filename, mime, dir)
		is.NoErr(err) // version 0

		err = db.Mv(context.TODO(), xid.NilID(), file, 0)
		is.NoErr(err) // version 1
	}))

}

func run(do func(t *testing.T, db *DB)) func(*testing.T) {
	return func(t *testing.T) {
		db, err := Up(context.TODO(), testFilename(t, "test.db"))
		if err != nil {
			t.Fatalf("running migrations on %s: %v", t.Name(), err)
		}
		t.Cleanup(func() {
			db.RWC.Close()
		})
		do(t, db)
	}
}

func Test_Up(t *testing.T) {
	is := is.NewRelaxed(t)
	_, err := Up(context.TODO(), testFilename(t, ""))
	is.NoErr(err)
}

func testFilename(t testing.TB, filename string) string {
	t.Helper()
	if filename == "" {
		filename = "test.db"
	}
	if os.Getenv("DEBUG") != "1" {
		return filepath.Join(t.TempDir(), filename)
	}
	_ = os.Remove(filename)
	return filepath.Join(filename)
}
