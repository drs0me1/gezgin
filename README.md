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

- Sign-in: only the `json` method (username and password) remains; `noauth`, `hook`, `proxy` and
  reCAPTCHA are removed. There is no self-registration: an admin creates every account.
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
  back with a restore. The Trash page shows the items in the same view as a folder of files (tiles
  or list, icons, a folder's item count and size, the user's sorting, the deletion time in place
  of the modification time); an item there does not open, and its old place is its tooltip.
- "Yeni klasör" and "Yeni dosya" in the sidebar make the item in the folder open in "Dosyalarım";
  elsewhere they used to make it at the top of the user's files without a word, and now say why
  they cannot (in the trash: "Çöpte yeni klasör ya da dosya oluşturulamaz.").
- Viewer: the EPUB reader is removed; an `.epub` has no preview, like other files without one.
- Editor: Ace's modes, themes and workers ship with Gezgin (no CDN). Texts in UTF-8 and in the
  Turkish code page (Windows-1254) open decoded and save back in their encoding; a text no
  supported encoding reads back unchanged opens read-only. A save names the version the file was
  opened at and is refused (409, the editor asks before overwriting) when the file changed since;
  Ctrl+S without a change writes nothing. Windows-1254 subtitles reach the player as UTF-8.
- Uploads: a tus upload's data is staged in `<root>/.gezgin-yukleme/` (unreachable through any
  path) and the file is put in place, keeping a replaced file's permissions, only once it is
  complete; cancelling, abandoning (3 minutes without a chunk) or a restart drops the staged data
  and leaves the destination as it was, and an unfinished upload is never listed, downloaded or
  found. Cancelling takes the create permission instead of delete. An upload that does not fit on
  the disk is refused at the start (HTTP 507, "Diskte yeterli boş alan yok"). The Redis upload
  cache option (`redisCacheUrl`) is removed.
- Search: every word has to be in the name; letter case, Turkish letters and accents do not
  matter (`ışık`, `isik` and `IŞIK` find `Işık notları.txt`; `case:sensitive` still matches
  exactly); `type:` matches extensions in any case; folders the rules refuse are not searched.
- Downloads: without the download permission a file's content, preview, subtitles and checksum are
  refused with 403 (File Browser answered an empty 202); the download dialog offers zip, tar and
  tar.gz (the server still makes the other formats). A ZIP compresses what is not compressed
  already and keeps photos, videos, music and archives as they are (File Browser compressed
  nothing). Special files such as FIFOs (which could hang a download), Gezgin's own files, links
  out of the scope or into Gezgin's folders and links to a folder they lie in are left out. Ctrl+S
  opens the download dialog again. A folder download packs at most 10,000 files and folders, as an
  archive job does; a larger one is refused (422, with a message) before anything is sent.
- Transfers at the same time: at most 10 downloads (a file, a folder as an archive, a video being
  played) and 10 uploads per user, 10 downloads per share link and 10 transfers (GET or PUT) per
  WebDAV share. One more answers 429 with a message, which a download's tab shows; an upload
  retries by itself. Previews, thumbnails, subtitles and listings are not counted.
- Share links: a link follows the item it shares; a rename or move takes the links of every user
  along (a link whose owner cannot reach the new place ends), and a delete ends them all, so that
  whatever later takes the old place is never served through an old link. Deleting a user deletes
  their links and their trash. Wrong passwords of a protected link are limited like logins (5 per link and 20 per
  address in 15 minutes, then HTTP 429). Links are named by 96 random bits (16 characters). A
  duration must be a whole number of seconds, minutes, hours or days of at most 10 years (else
  HTTP 400); 0 means permanent, and the share dialog proposes 7 days. Only what the user may see
  can be shared. Listing links among expired ones no longer fails.
- WebDAV shares: a folder can also be shared over WebDAV, for Finder, Infuse and other apps, on a
  port of its own (`--webdavPort`, `FB_WEBDAV_PORT`; off when empty) that serves nothing else.
  The address is `http://<host>:<port>/<share id>/`, with the username and password the share was
  made with (Basic authentication; wrong passwords are limited like a link's). A share is
  read-only unless made read-write by a user who may create, change, rename and delete; then a
  write takes a file's place only when complete, a delete moves the item into the owner's trash,
  and a move takes the share links along. The owner's permissions and rules hold, Gezgin's own
  files are not shown, and a symbolic link does not lead out of the shared folder. The share
  dialog offers it for folders when the port is set; it lasts 7 days unless told otherwise.
- Shares page: "Paylaşılanlar", after "Sık kullanılanlar" in the sidebar for users who may share,
  replaces Settings → "Paylaşım yönetimi". A fixed list gives each share's item with its icon (it
  opens the item in its place) and folder, a lock when it has a password, its kind ("Bağlantı", or
  "WebDAV" read-only or read-write with its username), its end (amber within a day) and, for an
  admin, who made it; sorted by name or end. Under each item its address, with a copy button
  (a link's opens in a new tab); the gear changes the share's settings and the bin removes it. A change keeps the address (`PATCH /api/share/<id>`): a new duration
  counted from now (the same units and 10-year limit) or none; a link's password set, replaced or
  removed, a new one ending the downloads begun with the old; a WebDAV share's password replaced
  (it stays required) and the share made read-only or read-write (for an owner who may create,
  change, rename and delete). An admin sees and changes every user's shares, others their own.
- Archives: a user who may create opens ZIP, RAR, 7z and tar archives (plain, `.gz`, `.bz2`, `.xz`,
  `.zst`) on the server with "Arşivi aç"; a RAR set (`name.part1.rar`, `name.part1of3.rar`, or
  `name.rar` with `name.r00`, ...) opens by its `.rar` parts or `name.r00` to `name.r99`, and files
  only named like a part (`firmware.s19`, a split ZIP's `.z01`) stay files. Each archive opens into
  a new folder beside it (with a number when the name is taken), and the archives that come out of
  it into folders beside them, up to 5 layers; the archives themselves stay. Gezgin does it in Go,
  with no other program. A job runs in the background, one at a time, shows its progress and can be
  cancelled; its folder is built in `<root>/.gezgin-arsiv/` (unreachable through any path) and put
  in place only when complete. Encrypted RAR and 7z archives open with their password (encrypted
  ZIPs do not). A job stops and leaves nothing on a name that leads out of its folder or that Gezgin
  keeps, a link or special file, the same name twice, a missing RAR part, a damaged archive, a
  source that changes meanwhile, more than 10,000 files and folders, or when less than 1 GiB would
  be left free on the disk; the user's rules hold for what comes out. Decoding a RAR may take up to
  1 GiB of memory; a 7z takes what its dictionary asks. A ZIP, 7z or tar split by 7-Zip or Gezgin
  in volumes (`name.zip.001`, `name.zip.002`, ...) opens by any of them; a gap in their numbers,
  or a missing last volume, stops the job.
- Making archives: a user who may create and download packs files and folders of one folder into
  a ZIP, tar or tar.gz beside them with "Arşiv oluştur" (in the header, the context menu, and the
  "more" menu on phones), optionally in volumes of 25 MB, 100 MB, 1 GB, 4 GB (FAT32) or a size of
  their own (`name.zip.001`, `name.zip.002`, ..., raw slices of the archive that 7-Zip and Gezgin
  open and `copy /b` or `cat` join). One folder's contents go at the archive's root. A ZIP
  compresses what is not compressed already; its times are written in the browser's time zone,
  which Windows shows them in, and a file of 4 GiB or more gets ZIP64 sizes in its local header
  too, so that 7-Zip reads it without a header error. RAR and 7z are not made (RAR's compression
  is proprietary; Go has no 7z writer). The job runs like an extraction, one at a time, in
  `<root>/.gezgin-arsiv/`; the archive, or every volume of a set, is put in place only when
  complete, with a number when the name or a volume of a set of that name is taken. What the
  rules refuse, special files, links that lead out of the scope or into Gezgin's folders, a link to
  a folder it lies in, and what Gezgin's own extraction would refuse the archive for (a name that
  is not UTF-8 or holds a control character, a chosen `.gezgin-` item, a file or folder more than
  64 folders deep) are left out and counted; names Windows would refuse are counted. A job stops on more than 10,000 files and folders, a file that changes
  while it is packed, more than 999 volumes, or when less than 1 GiB would be left free.
- Security headers: every answer carries a Content Security Policy, the page too, which had none
  (the router serves it as its not-found handler, which the header middleware never reached):
  scripts come only from Gezgin's own files, no other site may frame Gezgin, and
  `Referrer-Policy: same-origin` is sent. A user's file served as it is runs no script and may be
  framed by Gezgin alone, for the PDF preview. The page has no inline script: its settings come as
  a JSON data block and the web app manifest from `/manifest.webmanifest`. The old-browser
  (legacy) build is removed, 5 MB less: Gezgin needs a browser from about 2022 on. An unknown
  `/api/` path answers 404 instead of the page.
- Listings: archives and the parts of a set (`.r00`, `.zip.001`, `.part2.rar`, `.tgz`, ...) are
  listed as archives, and a file whose extension names a type other than text is no longer listed
  as text (both were, under 10 MB, as no header is read for a listing).
- File Browser's leftovers are removed: the sunset card on the settings page, the start-up notices
  (one line, "Gezgin <version>", instead), the contribution guide, code of conduct, issue and pull
  request templates, Transifex and PR-title workflow, the install docs and the s6 image variant.
  The image listens on 8080 by default and has no `HEALTHCHECK`, which an OCI image drops anyway.
- Every text of the interface is in Turkish, with "klasör" and "şifre" throughout.
- Folder sizes: a listing shows each folder's item count and size ("248 öğe · 1.82 GiB"), walked
  as the folder is listed, links not followed, within 200,000 entries and 2 seconds per listing
  (past them, only the count). The count is what the user sees on opening the folder; the size is
  everything the rules let them reach, hidden files included. The Info window gives a folder's
  count and size and a selection's real total (a folder used to count as its 4096-byte directory
  entry), and sorting by size orders folders by their size. Only the file listing and share pages
  ask for it (`?sizes=true`).
- Favourites: a star in the header, the context menu and the phone's selection bar marks a file
  or folder, at most 20 per user, kept on the server. The "Sık kullanılanlar" page in the sidebar
  shows them in the same view as a folder of files (tiles or list, icons, thumbnails, folder
  sizes, the user's sorting), each a shortcut that opens its item in its place, with its location
  as a tooltip and in the Info window; there an item is shared, downloaded or taken out of the
  favourites, and changed in its own folder. A favourite follows its item through Gezgin's renames
  and moves, WebDAV's too, and goes with a delete or when the item leaves the user's reach.
- Header: back, forward, up and home at its left, before the search bar, in "Dosyalarım", "Sık
  kullanılanlar", "Paylaşılanlar", "Çöp" and Settings. Back and forward move through Gezgin's own
  history in the tab and are dimmed at its ends, so they never leave Gezgin (signing in leaves
  no login page behind); up opens the parent folder with the folder left selected (Alt+↑, ⌘↑ on
  a Mac); home opens the top of "Dosyalarım". On a computer the header keeps, of an item's
  actions, only the favourite star: the others, download and Info are in the right-click menu,
  which opens anywhere in a folder and, off the items, is the folder's. A phone, without
  right-click, keeps them in its selection bar and ⋮ menu, shows back, up and home, and its
  selection bar spans the screen and wraps (an archive's "Sil" used to fall off it). The search
  bar narrows, down to 10em, before the header overflows. The buttons are drawn with thin
  outline icons (Tabler Icons, MIT), "Taşı" as a folder with an arrow, and the help window lists
  the keys, in Turkish.
- Gezgin's own logo, a pale blue folder, in place of File Browser's, with its favicons and app
  icons; the `branding` folder goes. The version is the short commit the image was built from
  ("Gezgin 5074d927" in the sidebar, the log and `filebrowser version`), and the command line
  help names Gezgin.
- Command runner and hooks: removed (File Browser kept them off by default as unsafe). There is no
  terminal, `/api/command`, execute permission, per-user command list, `shell` or `commands`
  setting, command on file events or `cmds` command, and Gezgin starts no other program.
  `--disableExec` is still accepted and changes nothing; settings and users saved with these
  fields load as before, without them.
- The container image keeps generated thumbnails in `/database/cache` (`FB_CACHE_DIR`).
- Passwords need at least 8 letters by default (File Browser: 12 bytes); the admin can set 8 to 32
  in the global settings, and a letter such as "ğ" counts once. A password over 72 bytes, which
  bcrypt cannot take, is refused with HTTP 400 (a share link's answered 500). Quick setup's
  generated admin password stays 16 characters.
- Global settings: a value out of bounds is refused when saved (HTTP 400; `config set` refuses it
  too): the minimum password length is 8 to 32, the upload chunk size 1 MB to 1 GB (a size of 0
  had browsers send empty chunks without end), the retries 0 to 20, and the default language,
  view mode and theme must be known values. No scope, home base folder or default scope can lie
  in Gezgin's folders (`.gezgin-cop`, `.gezgin-yukleme`, `.gezgin-arsiv`); a user scoped there was
  created and then refused everything. The defaults never grant the admin permission, and share
  comes with download. The brand is fixed: the instance name, colour, branding folder (custom
  styles and images), "disable external links", "hide the login button" and the custom logout
  page are removed; share pages never show a login link, logging out always opens the login
  screen, and error messages no longer offer to report an issue to File Browser. The theme and
  the disk usage bar stay, under "Appearance". The screen's texts are in Turkish, and a saved
  change to the theme, the disk bar or uploads reloads the page.

Details: [docs/authentication.md](docs/authentication.md).

The upstream README is kept in the history (`git show 833d9088:README.md`).

## License

Apache License 2.0, as the original. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
