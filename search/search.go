package search

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/rules"
)

type searchOptions struct {
	CaseSensitive bool
	Conditions    []condition
	Terms         []string
}

// Search searches for a query in a fs.
func Search(ctx context.Context,
	fs afero.Fs, scope, query string, checker rules.Checker, found func(path string, f os.FileInfo) error) error {
	search := parseSearch(query)

	scope = filepath.ToSlash(filepath.Clean(scope))
	scope = path.Join("/", scope)

	return afero.Walk(fs, scope, func(fPath string, f os.FileInfo, err error) error {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		if err != nil {
			// An entry that went away or cannot be read is not a result.
			return nil
		}
		fPath = filepath.ToSlash(filepath.Clean(fPath))
		fPath = path.Join("/", fPath)
		relativePath := strings.TrimPrefix(fPath, scope)
		relativePath = strings.TrimPrefix(relativePath, "/")

		if fPath == scope {
			return nil
		}

		if !checker.Check(fPath) {
			// Nothing in a refused folder is searched, as nothing in it is listed (Gezgin).
			if f.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if len(search.Conditions) > 0 {
			match := false

			for _, t := range search.Conditions {
				if t(fPath) {
					match = true
					break
				}
			}

			if !match {
				return nil
			}
		}

		if len(search.Terms) > 0 {
			_, fileName := path.Split(fPath)
			if !search.CaseSensitive {
				fileName = fold(fileName)
			}
			// Every term has to be in the name (Gezgin).
			for _, term := range search.Terms {
				if !strings.Contains(fileName, term) {
					return nil
				}
			}
		}

		return found(relativePath, f)
	})
}
