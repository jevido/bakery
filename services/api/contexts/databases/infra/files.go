package infra

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jevido/bakery/services/api/contexts/databases/app"
)

// BackupFiles keeps Backup files below Dir, in one directory per Database.
// Directories are 0700 and files 0600: they hold every row of a Database.
type BackupFiles struct{ Dir string }

func (f BackupFiles) dir(databaseID uint64) string {
	return filepath.Join(f.Dir, strconv.FormatUint(databaseID, 10))
}

func (f BackupFiles) path(databaseID uint64, name string) string {
	return filepath.Join(f.dir(databaseID), filepath.Base(name))
}

type counter struct {
	w io.Writer
	n int64
}

func (c *counter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// Write writes to name.partial, syncs it and renames it into place, so a
// Backup file is either complete or absent.
func (f BackupFiles) Write(databaseID uint64, name string, write func(w io.Writer) error) (int64, error) {
	if err := os.MkdirAll(f.dir(databaseID), 0o700); err != nil {
		return 0, err
	}
	final := f.path(databaseID, name)
	partial := final + ".partial"
	out, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	c := &counter{w: out}
	err = write(c)
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(partial, final)
	}
	if err != nil {
		os.Remove(partial)
		return 0, err
	}
	return c.n, nil
}

type backupFile struct {
	*os.File
	size int64
}

func (b backupFile) Size() int64 { return b.size }

func (f BackupFiles) Open(databaseID uint64, name string) (app.BackupFile, error) {
	file, err := os.Open(f.path(databaseID, name))
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	return backupFile{File: file, size: info.Size()}, nil
}

func (f BackupFiles) Remove(databaseID uint64, name string) error {
	err := os.Remove(f.path(databaseID, name))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (f BackupFiles) RemoveAll(databaseID uint64) error {
	return os.RemoveAll(f.dir(databaseID))
}
