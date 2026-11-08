package sqlite

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeStorageFixture(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte("existing contents"), mode); err != nil {
		t.Fatal(err)
	}
	// Set exact fixture permissions independently of the process umask.
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareStorageCreatesPrivateFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "nested", "sifeucr.sqlite")
	if err := prepareStorage(path, nil); err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		path string
		mode os.FileMode
	}{{path, 0o600}, {filepath.Dir(path), 0o700}, {filepath.Dir(filepath.Dir(path)), 0o700}} {
		info, err := os.Stat(entry.path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != entry.mode {
			t.Errorf("%s permissions = %04o, want %04o", entry.path, info.Mode().Perm(), entry.mode)
		}
	}

	writeStorageFixture(t, path, 0o600)
	if err := prepareStorage(path, nil); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "existing contents" {
		t.Fatalf("existing storage changed: %q, %v", contents, err)
	}
}

func TestPrepareStorageRejectsUnsafeFiles(t *testing.T) {
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		for _, kind := range []string{"permissions", "symlink", "directory"} {
			t.Run(kind+suffix, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "sifeucr.sqlite")
				if suffix != "" {
					writeStorageFixture(t, path, 0o600)
				}
				unsafePath := path + suffix
				switch kind {
				case "permissions":
					if runtime.GOOS == "windows" {
						t.Skip("Unix permission policy does not apply to Windows ACLs")
					}
					writeStorageFixture(t, unsafePath, 0o644)
				case "symlink":
					target := filepath.Join(t.TempDir(), "target.sqlite")
					writeStorageFixture(t, target, 0o600)
					if err := os.Symlink(target, unsafePath); err != nil {
						if runtime.GOOS == "windows" {
							t.Skipf("symlinks unavailable: %v", err)
						}
						t.Fatal(err)
					}
				case "directory":
					if err := os.Mkdir(unsafePath, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				if err := prepareStorage(path, nil); err == nil {
					t.Fatal("accepted unsafe SQLite storage")
				}
				info, err := os.Lstat(unsafePath)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "permissions" && info.Mode().Perm() != 0o644 {
					t.Fatal("validation silently changed existing permissions")
				}
				if kind == "symlink" && info.Mode()&os.ModeSymlink == 0 {
					t.Fatal("validation replaced an existing symlink")
				}
			})
		}
	}
}

func TestPrepareStorageMustExistModes(t *testing.T) {
	for _, mode := range []string{"ro", "rw"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "missing", "sifeucr.sqlite")
			params := url.Values{"mode": {mode}}
			if err := prepareStorage("file:"+filepath.ToSlash(path), params); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("missing storage: %v, want not-exist error", err)
			}
			if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("must-exist mode created a directory: %v", err)
			}
			path = filepath.Join(t.TempDir(), "existing.sqlite")
			writeStorageFixture(t, path, 0o600)
			if err := prepareStorage("file:"+filepath.ToSlash(path), params); err != nil {
				t.Fatalf("existing private storage: %v", err)
			}
		})
	}
}

func TestPrepareStorageURI(t *testing.T) {
	for _, form := range []string{"opaque", "escaped opaque", "localhost"} {
		t.Run(form, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "escaped ? name.sqlite")
			if form == "opaque" {
				path = filepath.Join(t.TempDir(), "plain name.sqlite")
			}
			filename := "file:" + filepath.ToSlash(path)
			switch form {
			case "escaped opaque":
				filename = "file:" + url.PathEscape(filepath.ToSlash(path))
			case "localhost":
				uriPath := filepath.ToSlash(path)
				if runtime.GOOS == "windows" {
					uriPath = "/" + uriPath
				}
				filename = (&url.URL{Scheme: "file", Host: "localhost", Path: uriPath}).String()
			}
			if err := prepareStorage(filename, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("URI did not create the expected file: %v", err)
			}
		})
	}
}

func TestPrepareStorageRejectsInvalidURI(t *testing.T) {
	for _, test := range []struct {
		name, filename string
		params         url.Values
	}{
		{name: "authority", filename: "file://remote/sifeucr.sqlite"},
		{name: "user info", filename: "file://user@localhost/sifeucr.sqlite"},
		{name: "malformed URL", filename: "file://[invalid"},
		{name: "invalid escape", filename: "file:bad%GG"},
		{name: "invalid mode", params: url.Values{"mode": {"invalid"}}},
		{name: "modeof", params: url.Values{"modeof": {"source.sqlite"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sifeucr.sqlite")
			filename := test.filename
			if filename == "" {
				filename = "file:" + filepath.ToSlash(path)
			}
			if err := prepareStorage(filename, test.params); err == nil {
				t.Fatal("accepted an invalid storage URI")
			}
			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid URI created storage: %v", err)
			}
		})
	}
}

func TestPrepareStorageSkipsNonFilesystemModes(t *testing.T) {
	for _, test := range []struct {
		name   string
		params url.Values
	}{
		{name: "memory", params: url.Values{"mode": {"memory"}}},
		{name: "custom VFS", params: url.Values{"vfs": {"custom"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "missing", "sifeucr.sqlite")
			if err := prepareStorage("file:"+filepath.ToSlash(path), test.params); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("non-filesystem mode created a directory: %v", err)
			}
		})
	}
	for _, filename := range []string{"", ":memory:", "file::memory:"} {
		if err := prepareStorage(filename, nil); err != nil {
			t.Fatalf("prepareStorage(%q): %v", filename, err)
		}
	}
}

func TestSQLiteOpensEscapedURI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "escaped ? name.sqlite")
	db, err := NewSQLite(t.Context(), "file:"+url.PathEscape(filepath.ToSlash(path)), WithForeignKeys())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(contents), "SQLite format 3") {
		t.Fatalf("expected a SQLite file at the decoded path: %v", err)
	}
}
