package fbhttp

import (
	"slices"
	"strings"
	"time"

	"github.com/filebrowser/filebrowser/v2/files"
	"github.com/filebrowser/filebrowser/v2/share"
	"github.com/filebrowser/filebrowser/v2/users"
)

// shareMark counts an item's shares in force: links and WebDAV shares (Gezgin, K115).
type shareMark struct {
	links, dav int
}

// shareMarks returns the shares in force by the path of their item in the user's scope: the
// user's own, or every user's for an admin, as the shares page lists them (K97); none for a user
// who may not share.
func (d *data) shareMarks() map[string]shareMark {
	if !d.user.Perm.Share && !d.user.Perm.Admin {
		return nil
	}
	var (
		links []*share.Link
		err   error
	)
	if d.user.Perm.Admin {
		links, err = d.store.Share.All()
	} else {
		links, err = d.store.Share.FindByUserID(d.user.ID)
	}
	if err != nil {
		// No shares, or the store failing: the marks are a help, and a listing does not fail
		// for them.
		return nil
	}

	now := time.Now().Unix()
	root := d.user.FullPath("/")
	owners := map[uint]*users.User{d.user.ID: d.user}
	marks := map[string]shareMark{}
	for _, l := range links {
		if l.Expire != 0 && l.Expire <= now {
			continue
		}
		owner, ok := owners[l.UserID]
		if !ok {
			if owner, err = d.store.Users.Get(d.server.Root, d.server.FollowExternalSymlinks, l.UserID); err != nil {
				owner = nil
			}
			owners[l.UserID] = owner
		}
		if owner == nil {
			continue
		}
		names, ok := d.inside(root, owner.FullPath(l.Path))
		if !ok {
			continue
		}
		p := "/" + strings.Join(names, "/")
		m := marks[p]
		if l.Kind == share.KindWebDAV {
			m.dav++
		} else {
			m.links++
		}
		marks[p] = m
	}
	return marks
}

// markItems marks the listed items the user keeps as favourites or has shared (K114-K116), so
// that the marks follow renames, moves, deletes and expiries.
func (d *data) markItems(items []*files.FileInfo) {
	marks := d.shareMarks()
	for _, item := range items {
		item.Favorite = slices.Contains(d.user.Favorites, item.Path)
		m := marks[item.Path]
		item.SharedLinks, item.SharedDAV = m.links, m.dav
	}
}
