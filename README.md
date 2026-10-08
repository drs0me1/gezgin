# Gezgin

Gezgin is a file manager for the [Konsol](https://github.com/drs0me1/myserver) server panel. It is a
modified fork of [File Browser](https://github.com/filebrowser/filebrowser) (Apache-2.0), which was
archived on 2026-09-01. The original history and licence are kept; see [NOTICE](NOTICE) and
[LICENSE](LICENSE).

## Status

Work in progress. Until the first Gezgin release this tree is File Browser's last state
(`v2.63.23`, commit `833d9088`) plus the changes listed in the commit history.

## Direction

- Runs as a Podman container on the tailnet, created through Konsol's container page.
- Keeps File Browser's interface, its own login screen and multi-user accounts with an admin; its
  forms stay even where Konsol has its own.
- Features of Konsol's Files module may be brought in where they help.
- Each part is reviewed with its endpoints before it is kept, changed or removed.
- Its own name and look; no File Browser branding in Gezgin builds.

## Changes from File Browser

- Sign-in: only the `json` (username and password) and `proxy` methods remain; `noauth`, `hook`
  and reCAPTCHA are removed.
- Password logins are limited per address and username (HTTP 429 with `Retry-After`).
- Sessions end when the password changes, with "Close all sessions" and when the user is deleted.
- The admin whose password quick setup generated chooses a new one at the first login.
- Users: the sole admin cannot lose the admin permission; only an admin deletes accounts; users
  change only their own preferences and password; usernames are unique in any letter case; rules
  are checked when saved (an expression must compile, a path must not be empty); a new scope gets
  its folder; the default language is Turkish.
- Files: an upload over a file and a save replace the file only once the new content is complete,
  keeping its permissions; a file and a folder never replace each other on a move or copy; a
  folder moved onto a folder of the same name is merged into it, as a copy already was; a move
  falls back to copying only between file systems and never touches the destination when that
  fails; listings take an entry's type from its extension (the `disableTypeDetectionByHeader`
  option is gone).
- Trash: a delete moves the item into the user's trash (`<root>/.gezgin-cop/<user id>/`, on the
  same disk, unreachable through any path); the Trash page restores items (into the root when
  their folder is gone, with a number when the name is taken), deletes them for good or empties
  the trash. "Delete permanently" stays in the delete dialog, and is the only way for an item on
  another disk. Items expire after 30 days by default (global settings, 0 = never); an admin sees
  how much every trash holds and can empty them all. Shares of a trashed item end and do not come
  back with a restore.

Details: [docs/authentication.md](docs/authentication.md).

The upstream README is kept in the history (`git show 833d9088:README.md`).

## License

Apache License 2.0, as the original. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
