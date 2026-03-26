package pactmigrate

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Load walks fsys starting at root (use "." for the FS root), discovers .sql
// files, parses hybrid version names, reads contents, and returns migrations
// sorted by Timestamp, then ContentHash, then Title.
func Load(fsys fs.FS, root string) ([]Migration, error) {
	root = strings.TrimPrefix(root, "./")
	if root == "" {
		root = "."
	}

	var out []Migration
	var firstErr error
	err := fs.WalkDir(fsys, root, func(fpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".sql") {
			return nil
		}

		ts, hash, title, perr := ParseFilename(fpath)
		if perr != nil {
			firstErr = fmt.Errorf("pactmigrate: %w", perr)
			return fs.SkipAll
		}

		sqlBytes, rerr := fs.ReadFile(fsys, fpath)
		if rerr != nil {
			firstErr = fmt.Errorf("pactmigrate: read %q: %w", fpath, rerr)
			return fs.SkipAll
		}

		out = append(out, Migration{
			Timestamp:   ts,
			ContentHash: hash,
			Title:       title,
			Filename:    fpath,
			SQL:         sqlBytes,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pactmigrate: walk %q: %w", root, err)
	}
	if firstErr != nil {
		return nil, firstErr
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Timestamp != b.Timestamp {
			return a.Timestamp < b.Timestamp
		}
		if a.ContentHash != b.ContentHash {
			return a.ContentHash < b.ContentHash
		}
		return a.Title < b.Title
	})

	return out, nil
}

// BaseName returns the file name segment of a migration path for logging.
func BaseName(m Migration) string {
	return path.Base(m.Filename)
}
