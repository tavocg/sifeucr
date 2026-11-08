package sqlite

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func prepareStorage(filename string, params url.Values) error {
	path, create, err := parseStoragePath(filename, params)
	if err != nil || path == "" {
		return err
	}
	if create {
		if err := ensurePrivateFile(path); err != nil {
			return err
		}
	}
	return validateStorageFiles(path)
}

func parseStoragePath(filename string, params url.Values) (string, bool, error) {
	path, create := filename, true
	if strings.HasPrefix(filename, "file:") {
		uri, err := url.Parse(filename)
		if err != nil {
			return "", false, err
		}
		if uri.User != nil || (uri.Host != "" && uri.Host != "localhost") {
			return "", false, fmt.Errorf("unsupported SQLite file URI authority")
		}
		path = uri.Path
		if uri.Opaque != "" {
			path, err = url.PathUnescape(uri.Opaque)
			if err != nil {
				return "", false, err
			}
		}
		switch params.Get("mode") {
		case "memory":
			return "", false, nil
		case "ro", "rw":
			create = false
		case "", "rwc":
		default:
			return "", false, fmt.Errorf("invalid SQLite URI mode %q", params.Get("mode"))
		}
	}
	// These names do not represent persistent files. Custom VFS storage is
	// responsible for its own access policy, rather than host filesystem modes.
	if path == "" || path == ":memory:" || params.Get("vfs") != "" {
		return "", false, nil
	}
	if strings.HasPrefix(filename, "file:") && params.Get("modeof") != "" {
		return "", false, fmt.Errorf("SQLite URI modeof cannot override private storage permissions")
	}
	if runtime.GOOS == "windows" && len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:] // SQLite file:///C:/... names a Windows drive, not a root directory.
	}
	return path, create, nil
}

func ensurePrivateFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create SQLite directory %q: %w", filepath.Dir(path), err)
	}
	// Only open new files: opening/closing an existing database separately can
	// interfere with POSIX locks held by other connections.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create private SQLite storage %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close new SQLite storage %q: %w", path, err)
	}
	return nil
}

func validateStorageFiles(path string) error {
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		info, err := os.Lstat(path + suffix)
		if errors.Is(err, os.ErrNotExist) && suffix != "" {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect SQLite storage %q: %w", path+suffix, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("SQLite storage %q must be a regular file, not a symlink or device", path+suffix)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			return fmt.Errorf("SQLite storage %q has permissions %04o; stop the service and set permissions to 0600", path+suffix, info.Mode().Perm())
		}
	}
	return nil
}
