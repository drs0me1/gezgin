package fbhttp

import (
	"errors"
	"path/filepath"
	"strings"

	fberrors "github.com/filebrowser/filebrowser/v2/errors"
	"github.com/filebrowser/filebrowser/v2/share"
	"github.com/filebrowser/filebrowser/v2/users"
)

// A share link follows the item it shares (Gezgin): a move takes the links along, and a delete ends
// them, so that whatever later takes the old place is never served through them. Links are matched
// by the item's real path, which finds those of every user who shared it, whatever their scope.

// realParts splits a real path into its names.
func realParts(p string) []string {
	p = strings.Trim(filepath.ToSlash(filepath.Clean(p)), "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// inside reports whether the real path p is base or lies inside it, comparing names in the letter
// case the file system does, and returns the names that lead from base to p.
func (d *data) inside(base, p string) ([]string, bool) {
	baseParts, parts := realParts(base), realParts(p)
	if len(parts) < len(baseParts) {
		return nil, false
	}
	for i, name := range baseParts {
		if name != parts[i] && (!d.server.CaseInsensitiveFs || !strings.EqualFold(name, parts[i])) {
			return nil, false
		}
	}
	return parts[len(baseParts):], true
}

// sharesOf returns the links to the item at the real path item and to anything inside it, with
// their owners.
func (d *data) sharesOf(item string) ([]*share.Link, map[uint]*users.User, error) {
	links, err := d.store.Share.All()
	if errors.Is(err, fberrors.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	owners := map[uint]*users.User{}
	var found []*share.Link
	for _, link := range links {
		owner, ok := owners[link.UserID]
		if !ok {
			owner, err = d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, link.UserID)
			if err != nil && !errors.Is(err, fberrors.ErrNotExist) {
				return nil, nil, err
			}
			owners[link.UserID] = owner // nil when the user is gone
		}
		if owner == nil {
			continue
		}
		if _, ok := d.inside(item, owner.FullPath(link.Path)); ok {
			found = append(found, link)
		}
	}
	return found, owners, nil
}

// dropShares ends the links to the item at the user's path p and to anything inside it.
func dropShares(d *data, p string) error {
	links, _, err := d.sharesOf(d.user.FullPath(p))
	if err != nil {
		return err
	}
	var errs []error
	for _, link := range links {
		errs = append(errs, d.store.Share.Delete(link.Hash))
	}
	return errors.Join(errs...)
}

// moveShares points the links to the item moved from the user's path src to dst, and to anything
// inside it, at the new place. A link whose owner cannot reach the new place ends.
func moveShares(d *data, src, dst string) error {
	from, to := d.user.FullPath(src), d.user.FullPath(dst)
	links, owners, err := d.sharesOf(from)
	if err != nil {
		return err
	}

	var errs []error
	for _, link := range links {
		owner := owners[link.UserID]
		below, _ := d.inside(from, owner.FullPath(link.Path))
		moved := filepath.Join(append([]string{to}, below...)...)
		names, ok := d.inside(owner.FullPath("/"), moved)
		if !ok {
			errs = append(errs, d.store.Share.Delete(link.Hash))
			continue
		}
		link.Path = "/" + strings.Join(names, "/")
		errs = append(errs, d.store.Share.Save(link))
	}
	return errors.Join(errs...)
}
