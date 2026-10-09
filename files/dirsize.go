package files

import (
	"errors"
	"os"
	"path"
	"time"

	"github.com/spf13/afero"

	"github.com/filebrowser/filebrowser/v2/rules"
)

// A listing asked for its folders' sizes walks each of them within one budget per listing
// (Gezgin, K85), as Konsol does: past it, a folder keeps its count and its size is unknown.
var (
	dirWalkEntries = 200000
	dirWalkTime    = 2 * time.Second
)

var errWalkBudget = errors.New("the walk ran out of its budget")

// dirWalk is the budget of one listing's walks: what is left of its entries, and its deadline.
type dirWalk struct {
	fs       afero.Fs
	checker  rules.Checker
	left     int
	deadline time.Time
}

func newDirWalk(fs afero.Fs, checker rules.Checker) *dirWalk {
	return &dirWalk{fs: fs, checker: checker, left: dirWalkEntries, deadline: time.Now().Add(dirWalkTime)}
}

// size is the total size of the regular files in the folder p and below that the checker allows.
// It follows no link and enters no folder the checker refuses; a folder it cannot read counts as
// empty. It fails with errWalkBudget once the listing's budget is spent.
func (w *dirWalk) size(p string) (int64, error) {
	entries, err := readDir(w.fs, p)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, e := range entries {
		w.left--
		if w.left < 0 || time.Now().After(w.deadline) {
			return 0, errWalkBudget
		}
		child := path.Join(p, e.Name())
		if !w.checker.Check(child) {
			continue
		}
		switch {
		case e.Mode()&os.ModeSymlink != 0:
		case e.IsDir():
			s, err := w.size(child)
			if errors.Is(err, errWalkBudget) {
				return 0, err
			}
			total += s
		case e.Mode().IsRegular():
			total += e.Size()
		}
	}
	return total, nil
}

// countItems is how many items the listing of the folder p shows: the entries the checker allows,
// but a link whose target the scope refuses, which a listing leaves out.
func countItems(fs afero.Fs, checker rules.Checker, p string) (int, error) {
	entries, err := readDir(fs, p)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range entries {
		child := path.Join(p, e.Name())
		if !checker.Check(child) {
			continue
		}
		if IsSymlink(e.Mode()) {
			if _, err := fs.Stat(child); errors.Is(err, os.ErrPermission) {
				continue
			}
		}
		n++
	}
	return n, nil
}

// setDirFacts gives the listed folder file its count and, unless it is a link or the budget is
// spent, its size (K84-K86). The count follows the listing's checker; the size, sizes, which also
// lets in the dotfiles a user hides.
func (file *FileInfo) setDirFacts(listChecker rules.Checker, walk *dirWalk) {
	if n, err := countItems(file.Fs, listChecker, file.Path); err == nil {
		file.Count = &n
	}
	file.Size, file.SizeUnknown = 0, true
	if file.IsSymlink || file.Count == nil {
		return
	}
	if total, err := walk.size(file.Path); err == nil {
		file.Size, file.SizeUnknown = total, false
	}
}
