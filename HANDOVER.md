# Handover — 2026-10-09

The work moves from a cloud session to a local session on the operator's MacBook. Run `git pull`
first, then read this file top to bottom: it holds the stages of the work, where each heading
stands, every decision taken so far and the tasks left. Tick tasks off, add new ones, and update
the decision list as decisions are taken. Speak Turkish to the operator; code comments, docs and
commit messages stay in English. Gezgin's headings keep their Turkish names, as used with the
operator.

## 1. Stages of the work

| Stage | What | State |
|---|---|---|
| A | Konsol's Files module (myserver repository, before Gezgin): folder tiles with item count and size (DD-247), folder sizes in "Sistem (/)" (DD-248), a CodeMirror text editor (DD-249), favourite folders (DD-250), container logs unfiltered (DD-251); v2-229 to v2-234 | Done, myserver `main` (`6845d51`) |
| B | Gezgin is started: File Browser v2.63.23 (archived upstream) forked into `drs0me1/gezgin`, named Gezgin, run as a Podman container, multi-user with an admin account, keeping its interface, login screen and forms. Konsol's Files module stays as it is; its features come over where they help. Each heading is reviewed with its endpoints and decided as we go | Done (`6f9fdfec`) |
| C | Image and deployment: GHCR workflow (`1e5d96ee`); container `gezgin` on the test host nrm, made through Konsol's container API (tailnet port 8091 to 8080, volumes `gezgin-db` and `gezgin-config`, the media folder bound to `/srv`); later the WebDAV port 8092 | Done |
| D | Heading-by-heading review: report the endpoints and verified problems, propose numbered decisions (K1, K2, ...), the operator decides, then implement, test, push, update nrm and verify live | **Current stage.** Headings 1-7 done; heading 7 waits only for the real-client trials (4.1); headings 8-10 left |
| E | Later work ("ileride"), outside the headings | Listed in section 4.5 |

**Where we stopped:** heading 7, "Komut çalıştırma". We analysed the command runner, removed the
shell and the file-event hooks (K45-K46), and in their place added archive extraction and creation
that run in Go inside Gezgin (K47-K58). The last step, creation (K54-K58), is pushed (`601ddbaa`)
and verified live on nrm. On 2026-10-09 a local session fixed its three name checks, reviewed the
archive work, removed the EPUB reader, self-signup and proxy sign-in (K59-K61), limited transfers
(K64-K65), and verified it all live on nrm (`45c22b38`); only the real-client trials of 4.1
remain before heading 7 is closed. Heading 8, "Yönetim ayarları ekranı", was then reviewed, its
proposals K66-K75 accepted as recommended, implemented (`a71c0a2e`), deployed on nrm and verified
live there. Heading 9, "Altyapı, marka ve CSP", was then reviewed, decided (K76-K83, the logo after
two drafts), implemented (`79805244`, `5074d927`) and deployed on nrm; its checks without sign-in
passed there, signed in too. Heading 10, "Konsol'dan alınacaklar", was then reviewed and decided
(K84-K89, K87 changed by the operator), implemented (`39ae53ce`) and verified live on nrm. The ten
headings of stage D are done; open are 4.1's Windows trials and the later work of 4.5.

## 2. Headings

| # | Heading | Decisions | Commits | State |
|---|---|---|---|---|
| 1 | Giriş ve oturum | K1-K6 | `2f1868bd` | Done, verified live |
| 2 | Kullanıcılar ve izinler | K7-K13 | `62ca5ee3` | Done, verified live |
| 3 | Dosya işlemleri | K14-K19, trash T1-T5 | `d8bf42e1`, `f914f099` | Done, verified live |
| 4 | Görüntüleme ve düzenleme | K20-K23 | `1c88e247`, `c1137664` | Done, verified live |
| 5 | Yükleme, indirme ve arama | K24-K31 | `36ad2a06` | Done, verified live |
| 6 | Paylaşım (links and WebDAV) | K32-K44 | `c8da81f8`, `aaf43505`, `403d332e` | Done, verified live |
| 7 | Komut çalıştırma, and archives in its place | K45-K58 | `116c3549`, `b0bb8b63`, `527111be`, `6d531015`, `601ddbaa` | Last step's follow-ups open (4.1) |
| 8 | Yönetim ayarları ekranı | K66-K75 | `a71c0a2e` | Done, verified live |
| 9 | Altyapı, marka ve CSP | K76-K83 | `79805244`, `5074d927`, `c11a6b80` | Done, verified live |
| 10 | Konsol'dan alınacaklar | K84-K89 | `39ae53ce` | Done, verified live |

Commit messages and README's "Changes from File Browser" describe each change.

## 3. Decisions taken

The next decision number is **K130** (K130-K134 are proposed, see 4.3). "Recommended" means the operator accepted the recommendation
made in the report.

**Stage B (start).** Multi-user with an admin; File Browser's forms stay even where Konsol has its
own; the Files module of Konsol is not touched; decide heading by heading, not all at once. Of the
four opening questions only multi-user was answered then: share links were later reviewed and kept
(heading 6), proxy sign-in is K1 (removed by K61), and the EPUB reader was removed by K59.

**1 — Giriş ve oturum** (all recommended)
- K1: only `json` and `proxy` sign-in remain; `noauth` and `hook` removed (a database using them
  refuses to start and says how to fix it). `proxy` stays until sign-on with Konsol is decided.
- K2: option (a), a per-user security stamp: a password change, "Tüm oturumları kapat" or deleting
  the user ends every session; a deleted user's token gets 401. Option (b), a server-side session
  table with per-device logout, is deferred.
- K3: login attempts limited, 5 per address and username and 20 per address in 15 minutes, then
  429 with `Retry-After`.
- K4: reCAPTCHA removed.
- K5: self-signup kept as File Browser had it, off by default (see 4.4).
- K6: the admin password quick setup generates must be changed at the first login.

**2 — Kullanıcılar ve izinler** (all recommended)
- K7: the only admin cannot lose the admin permission (form and CLI).
- K8: only an admin deletes accounts.
- K9: rules are checked when saved: a regex must compile, a path must not be empty (else 400).
- K10: the fields a user may change on their own account, and those an admin may change, are
  allowlists; the lock-password check is fixed.
- K11: usernames are unique in any letter case; 409 "Kullanıcı adı mevcut".
- K12: changing a user's scope creates its folder.
- K13: the default language is Turkish.

**3 — Dosya işlemleri** (all recommended; trash design approved as proposed)
- K14: an upload over a file and a save write a temporary file and replace the file only when
  complete, keeping its permissions.
- K15: a file and a folder never replace each other (409); copy-and-delete is used only across
  disks and never touches the destination when it fails.
- K16 (a): a folder moved onto a folder of the same name merges into it, and the dialog says so.
- K17: a trash. T1: `<root>/.gezgin-cop/<user id>/`, unreachable through any path. T2: items
  expire after 30 days by default (global setting, 0 = never). T3: each user handles their own
  trash; the admin sees every trash's size and can empty them all. T4: "Çöpe taşı" by default,
  "Kalıcı sil" beside it; an item on another disk can only be deleted permanently; a restore puts
  the item back, into the root when its folder is gone, with a number when the name is taken.
  T5: a trashed item's share links end and do not come back with a restore.
- K18: listings take an entry's type from its extension; `disableTypeDetectionByHeader` removed.
- K19: the conflict dialogs are in Turkish.

**4 — Görüntüleme ve düzenleme** (all recommended)
- K20: Ace's modes, themes and workers ship with Gezgin; no CDN.
- K21 (b): UTF-8 and Windows-1254 texts open decoded and save back in their encoding; others open
  read-only; Windows-1254 subtitles reach the player as UTF-8.
- K22: a save names the version opened; a file changed meanwhile gives 409 and an overwrite
  prompt; Ctrl+S without a change writes nothing.
- K23: thumbnails are cached in `/database/cache` (`FB_CACHE_DIR`).
- The operator's own requests: tests run on the main `gezgin` container on nrm, not throwaway
  ones; the minimum password length is 8 by default (`c1137664`).

**5 — Yükleme, indirme ve arama** (all recommended)
- K24: uploads are staged in `<root>/.gezgin-yukleme/` and put in place only when complete;
  cancelling, abandoning (3 minutes) or a restart drops the staged data only; cancelling needs the
  create permission.
- K25: an upload that does not fit is refused at the start (507).
- K26: the Redis upload cache is removed.
- K27: search needs every word in the name.
- K28: search ignores letter case, Turkish letters and accents; `case:sensitive` matches exactly.
- K29: search does not enter folders the rules refuse.
- K30: without the download permission, 403 instead of an empty 202.
- K31: the download dialog offers zip, tar and tar.gz; the API keeps the other formats.

**6 — Paylaşım** (all recommended; K32-K39 first, then the user's trash, then WebDAV)
- K32: pruning expired links no longer crashes the list or keeps expired links.
- K33: a link follows its item through renames and moves, other users' links too; it ends when the
  item leaves its owner's reach or is deleted; overwriting the file keeps it.
- K34: deleting a user deletes their links, and (approved follow-up, `aaf43505`) their trash.
- K35: wrong passwords of a protected link are limited: 5 per link and 20 per address in 15
  minutes, then 429.
- K36: link IDs have 16 characters (96 bits); old links keep working.
- K37: a duration must be a whole number of units, at most 10 years, else 400.
- K38: the share dialog proposes 7 days; 0 means permanent.
- K39: rules are checked when a link is made; refused paths, the trash and uploads give 403.
- K40: WebDAV shares of folders on a port of their own (`--webdavPort`, `FB_WEBDAV_PORT`; 8092 in
  the container and on the tailnet, instead of the planned 8081, so that the copied address is
  right), at `/<share id>/`, built on `golang.org/x/net/webdav`; rclone is not embedded.
- K41: each WebDAV share has its own username (proposed from the folder name) and a mandatory
  password of at least 8 characters; wrong passwords are limited as in K35.
- K42: read-only by default; read-write only for a user who may create, change, rename and delete,
  with explicit consent; writes are atomic, deletes go to the owner's trash, rules hold,
  `.gezgin-*` stays hidden, locking works for Finder.
- K43: 7 days by default, 0 = permanent; Settings → Shares shows the type.
- K44: nrm's `gezgin` container got the tailnet port 8092 and `FB_WEBDAV_PORT=8092` through
  Konsol's API.

**7 — Komut çalıştırma, then archives**
- K45: the interactive shell is removed: `/api/command`, Shell.vue, the execute permission, the
  per-user command list; `gorilla/websocket` dropped.
- K46: file-event hooks are removed: the runner, the `shell` and `commands` settings, the `cmds`
  CLI, the hooks form, their docs; `go-shlex` dropped. `--disableExec` is accepted and ignored.
- (K45-K46 accepted once it was clear that nothing is lost today: copying, moving and the other
  file operations never depended on them. Opening ZIP and RAR is one of the project's goals, so an
  archive engine in Go takes the shell's place.)
- K47: archives open inside Gezgin, in pure Go, with no external program: ZIP, RAR (old `.rNN` and
  new `.partN` sets), 7z, tar (plain, gz, bz2, xz, zst); nested archives up to 5 layers; ZIPs that
  each hold a part of a RAR set open together.
- K48: creating archives on the server was first declined ("sunucuda zip işine girmeyelim");
  superseded by K54 when the operator asked for creation.
- K49: limits: the output must leave 1 GiB free on the disk, 10,000 files and folders, 5 layers, a
  cancel button instead of a time limit, one job at a time.
- K50: a password field for encrypted RAR and 7z; encrypted ZIPs are refused.
- K51: no ISO extraction; the focus is compressing and extracting.
- K52: retiring Konsol's own ZIP/RAR is a later myserver job (4.5).
- K53: files only named like RAR parts (`.s19`, `.z64`, a split ZIP's `.z01`, orphan parts) stay
  files; "Arşivi aç" is offered on `.rar`, `.partN.rar`, `.partNofM.rar` and `.r00`-`.r99`; a set
  must be complete.
- K54: ZIP, tar and tar.gz are made on the server; "Arşiv oluştur" is in the header, the context
  menu and the phone's "more" menu.
- K55: RAR and 7z are not made (RAR's compression is proprietary; Go has no 7z writer).
- K56: an archive can be split in raw volumes `name.zip.001`, ... (25 MB, 100 MB, 1 GB, 4 GB for
  FAT32, or a custom size); Gezgin opens `.zip.001`, `.7z.001` and split tar sets.
- K57: creating needs the create and download permissions; what the rules refuse is skipped, as in
  downloads, and counted.
- K58: download ZIPs compress what is not compressed already; a FIFO no longer hangs a download,
  Ctrl+S works again, Gezgin's temporary files stay out.

**Open questions of 4.4, decided 2026-10-09** (all recommended)
- K59: the EPUB reader is removed. It read only `.epub` (epub.js); e-books download like any file.
- K60: self-signup is removed: the setting, the login screen's "create an account" link and
  `/api/signup` (supersedes K5). A self-registered user would get the default scope, the whole
  root, with every permission but admin.
- K61: `proxy` sign-in is removed; only `json` remains (supersedes K1's "until sign-on with Konsol
  is decided"). Gezgin cannot tell that the header came from a proxy: anyone reaching its port
  could name any user, the admin too.
- K62: CI (`.github/workflows/ci.yaml`) runs on pushes to `main` instead of `master`.
- K63: the stage B `filebrowser` test container is removed from nrm with its volumes and image
  (done through Konsol's container API; the media folder is untouched).
- K64: transfers at the same time are limited to 10: downloads (`/api/raw`: a file, a folder as an
  archive, a video being played) and uploads (tus chunks) per user, downloads per share link, GET
  and PUT per WebDAV share. One more answers 429 with a Turkish message (a download's tab shows
  it; tus retries by itself). Previews, thumbnails, subtitles and listings are not counted. The
  operator asked for "about 10 at the same time" instead of a large number per download.
- K65: a folder download packs at most 10,000 files and folders, as archive jobs do (K49); a
  larger one answers 422 with a message before anything is sent.
- Konsol's Podman page suggestions (a failed container's last log line, a warning on ports below
  1024 for the Files account) wait until Gezgin's headings are done (4.5).

**8 — Yönetim ayarları ekranı** (all recommended, 2026-10-09). `Settings.Save` checks every value,
so the API (400) and `config set` refuse the same; unset values take the defaults.
- K66: the minimum password length is 8-32 and counts letters ("ğ" is one); a password over 72
  bytes, which bcrypt cannot take, answers 400 (a share link's answered 500).
- K67: the upload chunk size is 1 MiB-1 GiB (0 made the browser send empty chunks without end; a
  stored 0 reads back as the default), retries 0-20; the form refuses text it cannot read.
- K68: Gezgin's folders (`.gezgin-cop`, `-yukleme`, `-arsiv`, `settings.ReservedDirs`) are refused
  as a user scope, the home base folder and the default scope.
- K69: the defaults never grant admin (no box), share comes with download, the language, view mode
  and theme must be known values (`settings.Locales` matches the translation files, by a test).
- K70: a fixed brand: the instance name, colour, branding folder (custom CSS and images, served
  without sign-in from any server path) and "disable external links" are removed; the name is
  Gezgin; error toasts no longer offer "Sorun bildir". Theme and disk bar stay under "Görünüm".
- K71: `hideLoginButton` is removed; share pages show no login link.
- K72: the screen in Turkish, rule labels too; known server errors are translated wherever shown
  (`frontend/src/utils/serverErrors.ts`).
- K73: the "create user home directory" box stays once unticked and gives the scope back.
- K74: removed the global `hideDotfiles`, `authHook`, the logout page (`--auth.logoutPage`),
  `Auther.LoginPage`, `CreateUserHome`, `SaveProvisioned` and `GetByScope`.
- K75: a saved change to the theme, the disk bar or uploads reloads the page.

**9 — Altyapı, marka ve CSP** (2026-10-09; all recommended but K77's choice and K79's design)
- K76: every answer carries the headers, the page too (it had none: the router's not-found
  handler, which the middleware never reached; now `withSecurityHeaders` wraps the router):
  `pageCSP` (scripts from Gezgin's files only, `frame-ancestors 'none'`, workers and media from
  `blob:`, fonts from `data:`) and `Referrer-Policy: same-origin`; raw files `rawCSP`
  (`script-src 'none'`, framed by Gezgin alone, for the PDF preview); no `X-XSS-Protection`.
- K77: no inline script (settings as a JSON data block `#gezgin-settings`, the manifest at
  `/manifest.webmanifest`, built files found through `import.meta.url`). The operator chose to
  drop the legacy build entirely: browsers from about 2022 on.
- K78: an unknown `/api/` path answers 404.
- K79: Gezgin's logo is a plain pale blue folder (after a compass and an owl's eye were shown);
  icons made from it, `branding/` removed, the help and `version` name Gezgin.
- K80: File Browser's sunset card, start-up notices, upstream docs, templates and workflows go;
  `SECURITY.md`, `docs/README.md` and `.claude/CLAUDE.md` are Gezgin's.
- K81: the image listens on 8080; no `HEALTHCHECK`, `JSON.sh` or s6 variant.
- K82: listings type archives and set parts as `archive`, and non-text extensions as `blob`.
- K83: every interface text in Turkish; "klasör" and "şifre" throughout.

**10 — Konsol'dan alınacaklar** (2026-10-09)
- K84 (recommended): folder tiles and rows read "N öğe · boyut" (an empty folder "0 öğe", one
  not walked only its count); the Info window gives a folder's count and size and a selection's
  real total; the size sort orders folders by their size.
- K85 (recommended): the listing walks each subfolder, links not followed, within 200,000
  entries and 2 seconds per listing (`files/dirsize.go`); only the listings shown ask for it
  (`?sizes=true`: `Files.vue`, `Share.vue`, `NewDir.vue`); no cache.
- K86 (recommended): the count follows the listing's check (rules, Gezgin's own items, dotfiles
  when hidden); the size, `CheckRules` (dotfiles included); refused folders are not entered.
- K87 (changed by the operator): one fixed "Sık kullanılanlar" page in the sidebar lists the
  favourite files and folders; no entry per item in the sidebar. A star in the header, the
  context menu and the phone's selection bar marks the one selected item; kept per user in the
  user record (`Favorites`), at most 20, through `/api/favorites` (add, remove, list).
- K88 (recommended): a favourite follows its item through a rename or move in Gezgin (WebDAV
  too), everyone's, and goes with a delete or trash, or when its owner can no longer reach it.
- K89 (operator): no change: sizes keep the universal format ("5.01 GiB"), not Turkish.
- K90 (recommended, after the operator asked for the page to look like "Dosyalarım" and
  suggested a hidden folder with links): the favourites page is a folder view the server builds
  from the favourites list (`files.ListingItems`, `/api/favorites?sizes=true`), with the same
  tiles, icons, thumbnails, folder facts, view mode and sorting; each tile is a shortcut
  (`ListingItem`'s `shortcut`) that opens its item in place, its location in the tooltip and the
  Info window. No folder or links on disk.
- K91 (recommended): there, an item is opened, shared, downloaded, looked at, or taken out with the
  star; rename, move, copy, delete and archive are done in its own folder; no drag and drop.
- K92 (operator): the Trash page uses the same view: tiles or list, icons by type (`/api/trash`
  gives `type` by name, `files.NameType`, and a folder's `count`), the user's sorting and view
  mode, the deletion time in place of the modification time, the old place as tooltip and in the
  Info window ("Eski yeri"); an item there does not open, has no thumbnail or checksum and is not
  dragged (`ListingItem`'s `trashed`); restore, delete for good and the two-step empty stay, in a
  bar above the items, the header, the phone's bar and the context menu.
- K93 (operator, widened): "Yeni klasör" and "Yeni dosya" in the sidebar are refused with a clear
  message in the trash; also, as they silently made the item at the top of the user's files from
  any page that is not a folder of "Dosyalarım", in favourites and on every other page, each with
  its own message.
- K94 (operator, as proposed with a mock-up): a "Paylaşılanlar" page in the sidebar after "Sık
  kullanılanlar", for users who may share, replaces Settings → "Paylaşım yönetimi"
  (`/settings/shares` leads to `/shares`); "Yeni klasör" there gives its own message.
- K95 (operator): a fixed list: the item's name with its icon, opening it in place, its folder
  under it, a lock for a password; its kind ("Bağlantı", "WebDAV · salt okunur/okuma-yazma" with
  its username); its end ("5 gün sonra", "Süresiz", "Süresi doldu", amber within a day);
  "Paylaşan" for an admin; sorted by name or end; per row copy, edit, remove. Narrower than 1024
  pixels the kind and owner go under the name, on a phone the end too (its sort beside the name's).
  `/api/shares` gives each share's `name`, `isDir`, `type`, `folder`, `open` (the item's path in
  the asker's scope, when they reach it) and `owner` (when not the asker).
- K96 (operator): `PATCH /api/share/<id>` changes a share, its address kept: a new duration from
  now (K37's units and limit) or none; a link's password set, replaced (a new token, so downloads
  begun with the old one end) or removed; a WebDAV share's password replaced, held to the account
  rules, never removed; read-only or read-write, the latter only for an owner who may create,
  change, rename and delete (else 403).
- K97 (operator): an admin sees and changes every user's shares, others only their own (403).
- K98 (operator, after seeing the page): each share's address is shown under its item, a link's
  opening in a new tab, with the copy button beside it (no longer in the row's buttons); copying
  uses the universal copy icon (`content_copy`) there, in the share dialog and on the public share
  page, in place of the clipboard (`content_paste`); the edit button is a gear (`settings`),
  "Paylaşım ayarları", as is its window's title.

- K99 (operator, from three mock-ups): back, forward, up and home as plain buttons (no
  background) at the header's left, before the search bar, in "Dosyalarım", "Sık kullanılanlar",
  "Paylaşılanlar", "Çöp" and Settings; not in the preview, the editor or the public share page
  (`HeaderBar`'s `showNav`, `header/Navigation.vue`).
- K100 (recommended): back and forward follow Gezgin's own history in the tab (the router's
  `history.state.back` and `forward`), dimmed at its ends so that they never leave Gezgin;
  signing in replaces the login page in the history; the browser's buttons, the mouse's side
  buttons and Alt+←/→ (⌘[ / ⌘] on a Mac) do the same.
- K101 (recommended): up opens the parent folder, which shows the folder left selected
  (`Files.vue` already selects the child it came from); dimmed at the top and off folders;
  Alt+↑ (⌘↑ on a Mac), not while typing or with a window open.
- K102 (recommended): home opens the top of "Dosyalarım", dimmed there; the breadcrumb's house
  stays.
- K103 (recommended): a phone shows back, up and home; forward is hidden.
- K104 (recommended, simplified by K109): the search bar narrows, down to 10em, before the
  header overflows; with K109 the header has at most four buttons on the right, so no ⋮ menu is
  needed on a computer.
- K105 (recommended): "Dosyayı taşı" is a folder with an arrow (`drive_file_move`), not the
  right arrow (`forward`) that would read as "İleri".
- K106 (operator: the first mock-up's icons): the buttons of the headers, the right-click menus
  and the phone's selection bars (`Action.vue`, through `Icon.vue`) and the search bar are drawn
  with thin outline icons, Tabler Icons 3.48.0 (MIT, its notice in `utils/icons.ts`), embedded
  as SVG paths keyed by the Material name, a favourite's star filled; a name not in the table
  keeps the Material font. The sidebar, the file icons and the dialogs keep theirs.
- K107 (operator: Info is enough in the right-click menu): Info leaves the header of
  "Dosyalarım", "Sık kullanılanlar" and "Çöp" on a computer; the right-click menu opens
  anywhere in the listing and in an empty folder, and off the items clears the selection, so
  that it is the folder's (Info, download); a phone keeps Info in its ⋮ menu (an iPhone has no
  right-click); the file preview keeps it (no right-click menu there).
- K108 (recommended): the phone's selection bar spans the screen, reads "1 seçili" and wraps;
  at 20em it took 325 pixels of 320 for a file and 368 for an archive, "Sil" off the screen.
- K109 (operator): on a computer the header drops what the right-click menu offers; of an
  item's actions only the favourite star stays (with the view, upload and multiple selection,
  which the menu does not have). Download goes too, Ctrl+S still downloads; in "Çöp" restore
  and delete for good stay in the menu and the bar above the items. A phone keeps its bar and ⋮.
- K110 (recommended): the search bar is typed in where it is; a click only puts the cursor in it;
  Enter searches, Esc or × empties it; keys typed in it no longer reach the folder (Delete, F2,
  Ctrl+A, Ctrl+S); Ctrl+Shift+F puts the cursor in it.
- K111 (operator chose B): the results are a page of their own (`/search/<folder>?q=`), a folder
  view of shortcuts like the favourites' (tiles or list, icons, thumbnails, the user's sorting
  applied in the browser as the server sorts, `utils/listingOrder.ts`), coming in as found with
  "Aranıyor…" and "Durdur", at most 500; the search sends each result's name, size, time, type
  and marks; a result's menu: "Bulunduğu klasörü aç" (the folder, the result selected), share,
  star, download, Info; back returns to the folder.
- K112 (operator, changed): no chips for the kinds: the query's conditions (`type:image`, ...)
  are enough. On a phone the magnifier opens the bar over the header, not a page.
- K113 (recommended): the sidebar scrolls up and down only, with a thin bar in the theme's
  colours; below 1024 pixels it is 12.75em, so "Sık kullanılanlar" fits beside the bar.
- K114 (recommended): a small yellow star after a favourite's name, in folders and search
  results, not on the favourites page.
- K115 (recommended): a small purple share sign after the name of an item with a share in force,
  its tooltip "Paylaşıldı: N bağlantı, N WebDAV"; a user's own shares, an admin's everyone's;
  the icon keeps its colour (videos are already that purple).
- K116 (recommended): the server marks each listed item (`favorite`, `sharedLinks`,
  `sharedDav`) in folders, favourites and search results; the star and the share dialog update
  the marks in place.
- K117 (operator, changed after the mock-up): the sidebar holds "Sık kullanılanlar" and
  "Paylaşılanlar" at the top, in view when it scrolls (`position: sticky`), a line under them;
  "Dosyalarım"; and "Çöp" at the bottom, above the disk use.
- K118 (operator): "Yeni klasör" and "Yeni dosya" at the top of the right-click menu on a
  folder's empty part and in an empty folder, for a user who may create, a line under them.
- K119 (operator, changed): they leave the sidebar; a phone, without right-click, has them in its
  ⋮ menu. The sidebar's messages for new items outside a folder (K93) go with them.
- K120 (operator: the user, settings and logout to the header's right): one account button at
  the header's right, the user's name beside the icon on a computer, the icon alone on a phone,
  opening "Profil ayarları", "Ayarlar" (an admin's) and "Çıkış" (`header/Account.vue`).
- K121 (operator): the trash's restore, delete for good and two-step empty leave the bar above
  the items (which keeps the count and size) for the right-click menu: an item's "Geri yükle",
  "Kalıcı sil"; off the items "Tümünü geri yükle" and "Tümünü sil", each asking first in a window
  (`prompts/Confirm.vue`; red for the delete, "Evet, hepsini kalıcı sil"); a phone has the two in
  its ⋮ menu.
- K122 (operator): the disk use goes over "Çöp" in a bottom part of the sidebar, with the
  version, in view when the sidebar scrolls and on every page (the user's files' disk off
  "Dosyalarım", the open folder's in it).
- K123 (operator): "Sık kullanılanlar" is "Favoriler" everywhere ("Favorilere ekle",
  "Favorilerden çıkar", "{ad} favorilere eklendi", the mark's "Favori", "En fazla 20 favori
  olabilir.").
- K124 (operator): "Dosyayı kopyala" and "Dosyayı taşı" are "Kopyala" and "Taşı".
- K125 (operator, changed after the mock-up: one icon, not three): the view button leaves the
  header and the phone's ⋮ for the right of the path row (`Breadcrumbs`' slot), and of the top
  line on the favourites (which get one: "N öğe"), trash and search pages
  (`files/ListingTools.vue`); it shows the view it goes to, now `layout-grid`, `layout-cards`
  and `layout-list`, no more the picture that read as images.
- K126 (recommended A, not objected to): rename is drawn as a text box with a cursor (`forms`).
- K127 (recommended A, not objected to): "Arşiv oluştur" as a zipped file (`file-zip`).
- K128 (recommended, not objected to): the right-click menu's "İndir" has no count.
- K129 (operator): "Seç", beside the view button, replaces the header's multiple selection: a
  tick circle on every item, a click ticking it (a second click does not open it), the button
  turning into "Bitti", which clears the selection and ends it, as do Esc and leaving the folder;
  the old "Çoklu seçim etkin" bar goes.

## 4. Tasks

### 4.1 Close heading 7: archive creation follow-ups (commit `601ddbaa`)

- [x] **C1 control characters.** `pack.archiveName` refuses only runes below 0x20 and 0x7f, while
  `unpack.clean` refuses every `unicode.IsControl` rune (also U+0080-U+009F). Such a name makes
  Gezgin's own "Arşivi aç" refuse the archive (`unsafe`). Use `unicode.IsControl` in
  `pack.archiveName` and `http.archiveName`; add tests.
- [x] **Depth of files.** `pack`'s `walk` checks `maxDepth` (64 parts) only for folders; a file in
  a folder 64 deep gets 65 parts and `unpack` refuses the archive. Check every entry in `add()`.
- [x] **Chosen `.gezgin-` items.** Such names are skipped inside folders, but an item chosen
  directly (an upload's `.gezgin-*.tmp`, which listings show) is packed and `unpack` refuses it.
  Refuse the prefix in `pack.archiveName` (skipped and counted).
  These three are done together, with `pack`'s `TestUnpackTakesItBack`, which opens each case
  with `unpack`. A file chosen alone whose name cannot be held is now skipped too, instead of
  going in unnamed.
- [x] Code review of the archive work (4.2), then fix what it confirms.
- [x] Redeploy to nrm and repeat the live checks (section 7). Done 2026-10-09 with `45c22b38`: 19
  API checks passed, section 7's (without the 4.7 GB ZIP64 entry and the browser ones, the writers
  being unchanged) and the new ones: no sign-up page or `/api/signup`, an `.epub` without a reader,
  a C1 name left out of an archive that Gezgin then opens, 10,001 entries refused (422) and an
  eleventh download at the same time refused (429), free again once the others ended.
- [x] On nrm, delete the test folder `/k54-deneme` permanently (it held a sparse 4.7 GB file).
- [ ] Try real clients: Windows 11 Explorer (a ZIP with Turkish names; times in local time), 7-Zip
  on Windows with a `.zip.001` set, an entry over 4 GiB on Windows, and a downloaded (streamed) ZIP
  in a reader that reads front to back (see 4.2's first note). macOS is done (2026-10-09): ditto,
  Archive Utility's engine, on macOS 27 opened a created and a streamed ZIP with Turkish names, an
  empty folder, the exec bit and the files' times intact.
- [ ] Report heading 7 as closed to the operator.

### 4.2 Reviews

- [x] **Archive work, `git diff 6d531015 601ddbaa`.** A review was started in the cloud session
  and stopped before it reported; done 2026-10-09 in the local session, inline. No defect in the
  ZIP fields, offsets, ZIP64 fields, end records, descriptors, patching across volumes and limits,
  in packing safety (links, reserved paths, loops, special files, `os.SameFile` at copy time), in
  the jobs, numbering, rollback and UI, or in unpack's volume sets, joined reader and K53 logic.
  Fixed: a folder download that failed under way ended as a whole response with the error text
  appended (now the connection breaks, so the browser shows a failed download), and one that
  failed before anything was sent came as an attachment holding the error (now an error answer).
  Noted, not changed: (1) a streamed ZIP's stored entries carry data descriptors, as Go's own
  writer does; readers that skip the central directory (Java's `ZipInputStream`) refuse them;
  (2) a crash while a set's volumes are moved into place can leave part of the set; (3) the
  archive dialog shows a generic error for a name the server (400) or the rules (403) refuse.
  Dimensions:
  - ZIP/tar/volume writers (`pack/zip.go`, `tar.go`, `output.go`): APPNOTE fields and offsets,
    ZIP64 extras (local: both sizes; central: only the fields that are 0xFFFFFFFF), end records,
    stream-mode data descriptors, patching across volumes, byte and volume limits;
  - packing safety (`pack/pack.go`, `http/archive.go` `packSource`, `http/raw.go`): links and the
    reserved real paths, loops, special files, change detection (`os.SameFile` and ctime), rules,
    names `unpack` takes back, downloads with no entry limit;
  - HTTP jobs and UI (`http/archive.go`, `Archive.vue`, `ArchiveJobs.vue`, i18n): validation,
    permissions, `publishFiles` numbering, stale volumes and rollback, the job JSON;
  - unpack (`unpack/names.go`, `formats.go`): K53's RAR part logic, volume sets, the joined reader,
    which errors become `missingPart`.
- [ ] **Each new heading** starts with its review: read the endpoints and the code, check on nrm,
  and report in Turkish with the verified problems and numbered decisions (K66 onwards).

### 4.3 Next headings

- [x] **8 — Yönetim ayarları ekranı.** Reviewed 2026-10-09 on a local build (the code of nrm's
  `45c22b38`): a chunk size of 0 made uploads send empty chunks without end (30,756 in 3 s); the
  password length took 1 or 100; users scoped in `.gezgin-arsiv`/`-yukleme` were created and then
  refused everything; the defaults took admin; `branding.files` served `img/` and `custom.css`
  from any server path without sign-in; 14 English texts; dead fields. Decided K66-K75 (section
  3), implemented in `a71c0a2e`: Go tests (`settings/validate_test.go`, `dir_test.go`,
  `http/settings_test.go`, `users/password_test.go`) and frontend tests (`size`,
  `serverErrors`); checked in the browser on a local build (Turkish screen, both chunk size
  errors, the reload, the home folder box).
- [x] **Live checks of heading 8 on nrm** (2026-10-09, `a71c0a2e`). Without sign-in: title Gezgin,
  no `custom.css` (`/static/custom.css` 404), the page's settings without the removed ones. Signed
  in, through the operator's session in Claude's built-in browser (section 6): the branding holds
  only the theme and the disk bar; nine refused values (password length 1 and 100, chunk size 0,
  21 retries, home base and default scope in Gezgin's folders, language, view mode, theme) answer
  400 with their messages and nothing is stored; a 74-byte share link password answers 400 and
  makes no link; nrm's own settings save back unchanged (200). On the screen: Turkish, "Görünüm"
  with the theme and disk bar, no "Yönetici" box in the defaults; a chunk size of `0` gives
  "Parça boyutu 1 MiB ile 1 GiB arasında olmalı."; `20 MB` saves and reloads the page with the
  new size in force; set back to 10 MB.
- [x] **9 — Altyapı, marka ve CSP.** Proposed: a CSP for the index page; the inline startup
  script moved to a file; Gezgin's name, logo, icons and PWA manifest; remove
  "File Browser (untracked)", the "Sorun bildir" and other upstream links and the "project
  archived" log lines; review the Turkish texts. Keep: one Go program with the interface built in,
  BoltDB, the CLI (`users` resets an admin password), `/health`, the 34 languages with Turkish by
  default, the local Material Icons and Roboto fonts; File Browser's own TLS and Unix socket stay
  off (Caddy is in front). Already done: Gezgin's own image on port 8080. Seen since, for this
  heading: listings type `.001`, `.r00`, `.tgz` and similar as plain files and small archives as
  text; unknown `/api/*` paths answer 200 with `index.html` instead of 404; the settings pages
  show File Browser's English "being archived" banner. The sidebar shows "Gezgin" with the
  version, still "(untracked)" in local builds.
  Reviewed 2026-10-09 (`http/http.go`, `static.go`, `raw.go`, `index.html`, the build, the image,
  the texts), checked on nrm (`a71c0a2e`, headers without sign-in) and on a local build. Verified:
  the page itself (`/`, `/login`, `/files/...`, `/share/...`) has no CSP and no frame protection,
  because it is the router's not-found handler, which the header middleware never reaches; for the
  same reason unknown `/api/*` paths answer 200 with the page. The page has two inline scripts of
  ours (settings, manifest) and four of `@vitejs/plugin-legacy`, whose old-browser bundle is 105
  files, 5.1 MB; Ace and the video player use `blob:` workers, the PDF preview an `<object>`. Raw
  files carry `script-src 'none'` (fine). Logo, favicons and PWA icons are File Browser's, the
  logos' alt text says "File Browser", `frontend/public/manifest.json` (unused) too; the sidebar
  reads "Gezgin gezgin-a71c0a2e". The settings page shows File Browser's English sunset card with
  a link to its GitHub; every start logs four "File Browser is being wound down" lines;
  `docs/README.md`, `SECURITY.md`, `CHANGELOG.md`, `transifex.yml`, `lint-pr.yaml`, `branding/`,
  `Dockerfile.s6` and `docker/s6` are upstream's, and `.claude/CLAUDE.md` is File Browser's
  advisory playbook (wrong project). The image listens on 80 by default (`EXPOSE 80`; nrm sets
  `FB_PORT=8080`), and nrm's container has no healthcheck (the OCI image drops `HEALTHCHECK`; the
  build downloads `JSON.sh` only for it). Listings call small archives and binaries "text"
  (`kucuk.zip`, `film.r00`, `film.zip.001`, `kod.tgz`, an ELF file) and those over 10 MB "blob";
  opening one finds the right type, so only the icon is wrong. Turkish: 4 texts missing (the
  sidebar's disk line among them) and 29 left in English; "dizin" and "klasör", "şifre" and
  "parola" both in use. Fine: one Go program, BoltDB, the CLI, `/health`, no request from the
  interface to another site but the sunset link, UID 1000 with no capabilities on nrm.
  Decided K76-K83 (section 3) and implemented in `79805244` and `5074d927`, with Go tests
  (`http/security_headers_test.go`, `files/listing_type_test.go`); checked in the browser on a
  local build: images, the PDF preview and Ace's worker under the CSP without a violation (data:
  icon fonts needed `font-src data:`). On nrm (`79805244`), without sign-in: the CSP and
  `Referrer-Policy` on every path, no inline script in the page, `/api/yok` 404, the manifest
  named Gezgin, `/static/custom.css` 404, one start-up line "Gezgin gezgin-79805244"; with
  `5074d927` the folder logo and the new icons are served.
- [x] **Signed-in live checks of heading 9 on nrm** (`c11a6b80`, the operator's session): the
  sidebar reads "Gezgin c11a6b80" under "5.01 GiB / 251 GiB kullanıldı", the header logo's alt
  text is Gezgin; the RAR set `rg-42386.rar`, `.r00`-`.r15` is listed as archives with the
  archive icon; the settings pages have no sunset card and the reworded permission texts; no CSP
  violation in the console while browsing (its only errors were heading 8's deliberate 400s). nrm
  holds no video, so the player was checked on a local build with a WebM made in the browser:
  video.js plays it under the CSP without an error.
- [x] **The image workflow's version line.** `.github/workflows/gezgin-image.yml` sets
  `version.Version=$short`, so that the sidebar reads "Gezgin <commit>" rather than "Gezgin
  gezgin-<commit>". The first push was refused for want of the `workflow` scope; the operator
  added it (`gh auth refresh -s workflow`).
- [x] **GitHub's private vulnerability reporting**, which `SECURITY.md` points to, is on (the
  operator agreed, 2026-10-09).
- [x] **10 — Konsol'dan alınacaklar.** Folder sizes and item counts on folder tiles (Konsol
  DD-247/248) and favourite folders (DD-250). Archive extraction and creation are done (7).
  Reviewed 2026-10-09 against Konsol (`drs0me1/myserver`, v2-229/230/232/233: DD-247, DD-248,
  DD-250). Konsol walks each subfolder of a listing for its size, links not followed, regular
  files summed, within one budget of 200,000 entries per listing; a folder past it, or
  unreadable, shows only its count ("248 öge · 1,82 TB"). DD-248 is its root view, which Gezgin
  has not. Favourites: a star, kept on the server (at most 20, 1024-byte paths), not following a
  rename. In Gezgin, verified: folder tiles and rows show "—"; the listing gives a folder the
  4096-byte size of its directory entry (nrm), which the Info window adds up for a selection
  (three folders read "12 KiB" though one holds 2.6 GB) and the size sort uses; no favourites;
  sizes are written "5.01 GiB", not in Turkish. Measured: a walk through Gezgin's file layer of
  101,000 entries takes 0.16-0.19 s (local SSD, warm); nrm's media folder holds 34 entries.
  Moves and deletes already call `moveShares`/`dropShares`, where favourites could follow too.
  Decided K84-K89 (section 3) and implemented: Go tests (`files/dirsize_test.go`,
  `http/dirsize_test.go`, `http/favorites_test.go`) and frontend tests (`folder`); checked in the
  browser on a local build: tiles read "4 öğe · 55 B", the Info window gives the folder's size and
  count, the star adds and the page lists, opens and removes favourites, no console error.
- [x] **Live checks of heading 10 on nrm** (`39ae53ce`, the operator's session): the tiles read
  "20 öğe · 1.6 GiB" for the Acronis folder, as on the disk (20 entries, 1,719,335,538 bytes),
  and "0 öğe" for the empty `movies` and `series`; a favourite added shows on the page and the ×
  takes it out (nothing left in the operator's list); no console error. Seen there: between 737
  and 1024 pixels wide the sidebar is 10em and cut "Sık kullanılanlar" (it needs 11.44em); it is
  12em now (`frontend/src/css/mobile.css`).
- [x] **Favourites as a folder view** (operator, 2026-10-09): decided K90-K91 (section 3) and
  implemented; Go test `TestFavoritesAsAFolderView`; checked in the browser on a local build: the
  page shows folders with "N öğe · size", a PDF, a video and an image with its thumbnail, a
  selected item's tooltip and Info give "Konum: /csp", its MD5 matches the file, the star takes it
  out, a double click opens the folder in place, and tiles cannot be dragged. Live on nrm
  (`c233903f`, the operator's session): a favourite added there shows as a mosaic tile "20 öğe ·
  1.6 GiB" with "Konum: Dosyalarım" as its tooltip, not draggable, and the star took it out again,
  leaving the operator's own favourite (`movies`); no CSP violation. The first image build of
  `c233903f` failed on a passing ghcr.io login error and passed when rerun.

- [x] **The Trash page in the folder view, and no new items outside a folder** (operator,
  2026-10-09): K92-K93 (section 3); Go test `TestTrashListTypesAndCounts`; checked in the browser
  on a local build: a trashed folder reads "1 öğe · 3 B", a PNG and a WebM their icons, a double
  click only selects, the Info window gives "Eski yeri" and "Silinme" without checksums, restore
  puts the PNG back, the empty takes two clicks; "Yeni klasör" in the trash says "Çöpte yeni
  klasör ya da dosya oluşturulamaz.", in favourites and settings their own messages, and still
  opens its prompt in a folder. Live on nrm (`2d157c4e`, the operator's session, looking only):
  the operator's trash reads "3 öğe · 840.38 MiB" in mosaic tiles, `rg-42386` "1 öğe · 840.38
  MiB" with its old place as tooltip; "Yeni klasör" there shows the message and opens nothing; no
  console error.

- [x] **A "Paylaşılanlar" page** (operator, 2026-10-09: after "Sık kullanılanlar" in the sidebar,
  the shares as a table, a fixed list view, shares editable; a design first): K94-K97 (section
  3). Before, the shares were only in Settings → "Paylaşım yönetimi" and a share could not be
  changed: changing one meant a new share and a new address. Go tests `TestShareEditLink`,
  `TestShareEditWebDAV`, `TestShareEditOwnersAndList`; frontend `utils/__tests__/shares.test.ts`.
  Checked in the browser on a local build with a WebDAV port: four shares of the admin and one of
  a user listed with icons, folders, locks, kinds, ends (amber within a day) and owners; sorting
  by name and end both ways; 5 days made 2 with a new password (the old one then refused, the new
  one let in); a WebDAV share made read-write for 30 days; 3700 days refused with "Bir paylaşım en
  fazla 10 yıl sürebilir."; a password removed and the link made permanent (it opened without
  one); a share removed after a question naming it; `/settings/shares` led to the page; "Yeni
  klasör" gave its message; no overflow at 841 and 375 pixels. Live on nrm (`9f88bee9`, the
  operator's session, looking only): the page lists the operator's one share, `movies`, with its
  lock, "WebDAV · salt okunur", "Kullanıcı: test", "Süresiz" and "Paylaşan: admin"; its edit
  window opens with nothing changed (end and password kept, read-write off) and was closed
  without saving; `/settings/shares` leads to the page; the settings tabs no longer have
  "Paylaşım yönetimi"; no console error.
  K98 (`b715caaa`): checked locally at 1280 and 375 pixels (the addresses wrap, no overflow;
  copying could not be tried there, as the built-in browser denies the clipboard to
  `127.0.0.1`). Live on nrm (`b715caaa`, the operator's session): `movies` shows its WebDAV
  address under it; the copy button beside it said "Link kopyalandı!"; the row keeps the gear
  and the bin; no console error.

- [x] **Back, forward, up and home in the header** (operator, 2026-10-09: beside the search bar,
  arrows that go back and forth as a file manager's do, and a home button; a design first):
  K99-K109 (section 3), after three mock-ups: the second showed every button of the header and,
  measured, two bugs of today (between 737 and 1030 pixels a selection pushed "Sil", download,
  upload, Info and multiple selection off the header; an archive's "Sil" fell off the phone's
  selection bar); the operator asked for no background colour, the first mock-up's thin icons,
  Info only in the right-click menu, and then for the header to drop everything the menu offers
  but the star. Frontend tests `utils/__tests__/navigation.test.ts` and `icons.test.ts`. Checked
  in the browser on a local build: at the top of "Dosyalarım" all four dimmed; in `albüm` back,
  up and home lit; up opened the top with `albüm` selected, back and forward walked the history
  and dimmed at its ends; ⌘↑ in `albüm/2024` opened `albüm` with `2024` selected; the header
  showed view, upload and multiple selection, and the star with an item selected; a right click
  on an item gave its nine actions, on the empty part or an empty folder only "İndir" and
  "Bilgi" with nothing selected, and "Bilgi" the folder's; Ctrl+S opened the download window;
  "Çöp", "Sık kullanılanlar", "Paylaşılanlar" and Settings had back and home lit, up dimmed; at
  800 pixels the header fits (the search 315 pixels with an archive selected); at 375 the phone
  shows back, up and home, its ⋮ view, download, upload, Info and multiple selection, and its
  selection bar "1 seçili" in one row for a file and two for an archive, nothing cut; the
  preview keeps its buttons, drawn thin with a shadow, and no arrows. Live on nrm (`ee65725e`,
  the operator's session, looking only): at the top of "Dosyalarım" all four dimmed and the
  header's right holding view, upload and multiple selection, eleven outline icons in it; in
  `movies` back, up and home lit; up opened the top with `movies` selected; a right click on the
  empty part gave "İndir" and "Bilgi" with nothing selected (closed unused); no console error.
  Seen with the eye there (screenshots, 860 by 640 and the pane's own phone width): the thin
  icons, the dimmed arrows, the line before the search bar, the phone's back, up and home; a
  right click on a folder gave its nine actions with the folder-and-arrow "Taşı" and the filled
  star in the header for a favourite (the menu was closed unused). The header still showed File
  Browser's floppy-disk logo: the server sent Gezgin's folder, but `/static/img/logo.svg` is
  cached for a day under the same name, so a browser kept the old one; fixed in `59d2f8ad` (the
  logo's, favicons' and manifest icons' addresses carry `?v=<version>`), live on nrm, the
  folder logo shown.
- [x] **Search without its menu** (operator, 2026-10-09: a click on the search bar opens a
  separate menu; type in the visible field and search instead). Today a click turns the bar
  into a full-page layer with the type boxes (images, music, video, PDF) and the results as a
  list of paths. Proposed with a mock-up, waiting: K110, a click only puts the cursor in the
  bar; Enter searches, Esc or × empties it; K111, the results either (A) in a list that drops
  from the bar, the page left in place, or (B, recommended) in the folder view, as Explorer
  shows them: tiles or list with icons, sizes and the user's sorting, each a shortcut opening
  its item in its place (as the favourites, K90), coming in as found with "Aranıyor…" and a
  stop, a page of its own so that ← returns to the folder (the search already reads each
  result's details; it would send name, size, time and type with the path); K112, the type
  boxes become chips above the results ("Tümü", "Görseller", "Müzik", "Video", "PDF"); on a
  phone the magnifier opens the bar in the header itself, not a page. Found in the screenshots,
  K113: under about 700 pixels of height the sidebar scrolls and shows a white vertical and a
  horizontal scrollbar in the dark theme, cutting "Sık kullanılanlar"; proposed: no sideways
  scroll and a thin scrollbar in the theme's colours.
- [x] **Marks for shared and favourite items** (operator, 2026-10-09: mark the shared and the
  favourite files and folders; the shared in the purple of sharing, the favourites with a star
  beside the name). Proposed with a mock-up, waiting: K114, a small yellow star right after the
  name of a favourite, in the folder view (tiles and list) and the search results, not on the
  favourites page, where all are; K115, a small purple share sign after the name of an item with
  a share in force (its tooltip: "Paylaşıldı", how many links and WebDAV shares), a user's own
  shares and an admin's everyone's, as on the shares page (K97); the item's own icon keeps its
  colour, as videos are already that purple (`--icon-violet`); K116, the server marks each
  listed item (`favorite`, `shares`) as it lists, so the marks follow renames, moves, deletes
  and expiries without another request.
  Both done as K110-K116 (section 3). Go tests `TestListingMarks` (favourite, two links, a WebDAV
  share, an expired share unmarked, the admin seeing a user's share, another user seeing none)
  and `TestSearchResultsDescribeTheirItems`; frontend `utils/__tests__/listingOrder.test.ts`.
  Checked in the browser on a local build: `albüm` and `tur` showed the star and the purple sign
  (in tiles and list; "Paylaşıldı: 1 bağlantı", "1 WebDAV"), the favourites page the sign only; a
  click on the search bar opened nothing; "png" and Enter gave 15 results with thumbnails as
  tiles, the header's star with one selected, a menu of five with outline icons; "Bulunduğu
  klasörü aç" opened `silinecek` with `foto.png` selected, back returned to the results with
  the query kept; at 375 pixels the magnifier opened the bar over the header and "tur" gave the
  folder with its marks; at 860 by 600 the sidebar had a thin dark scrollbar, no sideways one,
  and "Sık kullanılanlar" whole. Found on the way: coming from a page that is not a folder's,
  `Files.vue` skipped the preselection; it no longer needs the previous listing for it. Live on
  nrm (`091112f5`, the operator's session after they signed in again, looking only): the top of
  "Dosyalarım" showed the star beside `Acronis…` and the star and the purple sign beside
  `movies`; `/api/search/?query=movies` answered `movies` with `favorite` and one WebDAV share,
  its time and `isDir`. The results page itself was not opened there: the operator was using
  the pane.
- [x] **The sidebar's order, and new items from the right-click menu** (operator, 2026-10-09:
  the trash to the bottom of the last part; favourites and shares to the top, a fixed part
  set off by a line; "Yeni klasör" and "Yeni dosya" in the right-click menu inside a folder).
  Proposed with a mock-up, waiting: K117, at the top "Sık kullanılanlar" and "Paylaşılanlar",
  staying in view when the sidebar scrolls, a line under them; then the user and
  "Dosyalarım"; a line; "Yeni klasör", "Yeni dosya"; a line; "Ayarlar", "Çıkış" and last
  "Çöp", above the disk use; K118, "Yeni klasör" and "Yeni dosya" at the top of the right-click
  menu on a folder's empty part and in an empty folder, for a user who may create (an item's
  menu unchanged); K119, they stay in the sidebar too, for a phone, which has no right-click. The
  operator approved the right-click menu, took "Yeni klasör" and "Yeni dosya" out of the sidebar
  and moved the user, settings and logout to the header's right: K117-K120 (section 3). Checked
  in the browser on a local build: the sidebar with the two at the top, "Dosyalarım" under the
  line and "Çöp" at the bottom (computer and phone); the account button "yonetici" opening
  "Profil ayarları", "Ayarlar", "Çıkış"; a right click on `albüm`'s empty part giving "Yeni
  klasör", "Yeni dosya", a line, "İndir", "Bilgi", the first opening its window for `albüm`;
  at 375 pixels the header fits with the account's icon alone, and the ⋮ menu has "Yeni
  klasör" and "Yeni dosya". Found on the way: the new item window wrote a folder's name as the
  address has it ("alb%C3%BCm"); it now decodes it. Live on nrm (`8cecbb2e`, the operator's session in a
  tab of its own, looking only): the sidebar "Sık kullanılanlar", "Paylaşılanlar", "Dosyalarım",
  "Çöp" at the bottom; "admin" at the header's right opening "Profil ayarları", "Ayarlar",
  "Çıkış"; a right click on the empty part giving "Yeni klasör", "Yeni dosya", "İndir",
  "Bilgi"; both menus closed unused.
- [x] **Trash actions in the right-click menu, the disk use over the trash** (operator,
  2026-10-09, with a screenshot of the trash's bar): K121-K122 (section 3). Checked in the
  browser on a local build: the trash page with the count and size only; a right click on its
  empty part gave "Tümünü geri yükle", "Tümünü sil", "Bilgi"; the first asked "Çöpteki 3 öğe
  eski yerlerine geri yüklensin mi?" and put three test files back; the second asked in red and
  emptied the trash; the phone's ⋮ had both; the disk use over "Çöp" on the shares page, and at
  340 pixels of height the top and bottom parts stayed with "Dosyalarım" scrolling between. Live
  on nrm (`d2a48090`, the operator's session in a tab of its own, looking only): the trash page
  read "2 öğe · 1.64 GiB" with no buttons; a right click on its empty part gave "Tümünü geri
  yükle", "Tümünü sil", "Bilgi" (closed unused, the trash untouched); the sidebar showed
  "5.4 GiB / 251 GiB kullanıldı" on the trash page.
- [x] **Words and icons of the menus, the view switch** (operator, 2026-10-09, with two
  screenshots: the header with the view button and an empty spot at the right of the path row
  circled, and a folder's right-click menu; a design first). Proposed with a mock-up, waiting:
  K123, "Sık kullanılanlar" becomes "Favoriler" everywhere ("Favorilere ekle", "Favorilerden
  çıkar", "{ad} favorilere eklendi", the mark's "Favori", "En fazla 20 favori olabilir.");
  K124, "Dosyayı kopyala" and "Dosyayı taşı" become "Kopyala" and "Taşı"; K125, the view button
  (its "next: gallery" icon read as pictures) leaves the header for the right end of the path
  row, where the operator circled, as three icons (list, tiles, large tiles: `layout-list`,
  `layout-grid`, `layout-cards`), the one in use in the accent colour; on the pages without a
  path row at the right of their top line (the favourites get one with their count), the phone's
  ⋮ losing it; K126, "Yeniden adlandır" gets a text box with a cursor (`forms`, as Windows 11),
  or the I-beam (`cursor-text`); K127, "Arşiv oluştur" a zipped file (`file-zip`), or `zip`;
  K128, no count badge on "İndir" in the right-click menu (it covered the icon). The operator
  kept the view as one icon, beside it "Seç" (ticks to select): K123-K129 (section 3). Checked
  in the browser on a local build: the sidebar's "Favoriler", the page title "Favoriler -
  Gezgin" and its top line "4 öğe" with the view and "Seç"; in `albüm` the path row's right held
  the view button and "Seç", the header only upload and the account; "Seç" put five circles,
  ticked `2024` and `a.jpg` on a click each and read "Bitti", which cleared them; a right click
  gave "Favorilere ekle", "Yeniden adlandır" (text box), "Kopyala", "Taşı", "Arşiv oluştur"
  (zipped file), "İndir" without count; at 375 pixels the path row held both and the ⋮ menu
  neither. Found on the way: the right-click menu was placed before it was measured, so near the
  right edge it ran off the window and near the bottom its last items were cut; it now moves
  left, and up only as far as it must, never under the header. Live on nrm (`54d43511`, the operator's
  session in a tab of its own, looking only): the sidebar "Favoriler", "Paylaşılanlar",
  "Dosyalarım", "Çöp"; the path row's view button and "Seç"; a right click on `movies` gave
  "Paylaş", "Favorilerden çıkar", "Yeniden adlandır", "Kopyala", "Taşı", "Arşiv oluştur", "Sil",
  "İndir" (no count), "Bilgi", closed unused.
- [ ] **The shares page as a folder view, its actions in the right-click menu** (operator,
  2026-10-09: like the other pages, tiles or list switchable, the list's features in the
  right-click menu; a design first). Proposed with a mock-up, waiting: K130, one tile per share
  (an item shared twice shows twice), its item's icon, the name with a lock when it has a
  password, under it the kind ("Bağlantı", "WebDAV · salt okunur/okuma-yazma") and the end
  (amber within a day, "Süresiz", "Süresi doldu"), "Paylaşan: ali" for an admin when not theirs;
  the top line "N paylaşım" with the view button and "Seç"; the list view's columns Ad, Tür,
  Bitiş and an admin's Paylaşan, sorted by name or end; a double click opens the item in its
  place; K131, a share's right-click menu: "Bağlantıyı kopyala" (WebDAV: "Adresi kopyala"),
  "Yeni sekmede aç" (links), "Paylaşım ayarları", "Bulunduğu klasörü aç", a line, "Paylaşımı
  kaldır", "Bilgi"; with several selected "N paylaşımı kaldır", asking first; K132, the address
  leaves the tile, staying in the menu and the Info window; K133, the Info window gives the
  share: name, place, kind with access and WebDAV user, address, end, password or none, owner;
  K134, on a phone a tap selects and the bottom bar has copy, settings, remove and Info.

### 4.4 New decisions to put to the operator

- [x] The EPUB reader: removed (K59). Implementation: 4.6.
- [x] Self-signup: removed (K60). Implementation: 4.6.
- [x] `proxy` sign-in: removed (K61). Implementation: 4.6.
- [x] Downloads had no entry limit and planned every entry in memory first: K64 and K65.
- [x] CI runs on `main` (K62; changed in GitHub's web editor, `5fc96856`). Its first run on `main`
  passed every job, the race tests on Linux included.
- [x] The old `filebrowser` test container: removed from nrm (K63).
- [x] Konsol's Podman page suggestions: later (4.5).

### 4.5 Later ("ileride")

- [ ] K52: retire Konsol's own ZIP/RAR (myserver repository) once Gezgin's has proven itself.
- [ ] WebDAV shares from the internet through Konsol's HTTPS (port 8092 is plain HTTP, tailnet
  only); then perhaps retire Konsol's own WebDAV (port 61010).
- [ ] Video transcoding for formats browsers cannot play.
- [ ] K2 (b): a server-side session table with per-device logout.
- [ ] If Gezgin goes behind Konsol's Caddy, the per-address login limits (K3, K35) would count every
  request as one address: revisit them then.
- [ ] Only if needed: a webhook (without a shell) for upload notifications, fetching a URL to the
  server, checksums as a file action.
- [ ] Konsol's Podman page (myserver repository): show a failed container's last log line, and
  warn about ports below 1024 for a container that runs as the Files account.

### 4.6 Changes decided in 4.4

- [x] K59: remove the EPUB reader (`vue-reader`, `epubjs`, `epubReader.css`, Preview's branch).
- [x] K60: remove self-signup (setting, login link, `/api/signup`, CLI flag, i18n texts, docs).
- [x] K61: remove `proxy` sign-in (auther, CLI flags, docs); a database set to `proxy` refuses to
  start and says how to switch, as K1 did for `noauth` and `hook`. Checked with a database made by
  the previous build: refused, then `config set --auth.method json` and it serves.
- [x] K64, K65: the transfer limits (`http/transfers.go`, tests in `transfers_test.go`).
- [x] Later cleanup: `settings.CreateUserHome`, `users.Storage.SaveProvisioned` and `GetByScope`
  served only signup and proxy sign-in; removed with K74.

## 5. How to work

- Gezgin is a project of its own, apart from debian-server-installer (Konsol): its code, handover
  and decisions live in this repository only. Work in a session opened in Gezgin's own local clone,
  not in debian-server-installer's folder: claude-mem files a session's memory under the folder it
  was opened in, so Gezgin's memory stays under its own project, `gezgin`. The first local session
  ran in debian-server-installer's folder; its memory was moved with
  `claude-mem project merge debian-server-installer gezgin`.
- Commit and push to `main` unless the operator says otherwise.
- Per heading: review (4.2), report in Turkish with numbered decisions and a recommendation,
  implement once the operator decides, test, commit, push, wait for the image, update nrm, verify
  live, report what was run.
- nrm is a disposable test host: data loss is acceptable, make no server-side backups, test with
  the main `gezgin` container. Its admin password is the operator's test password, not written
  here. The repository is public: no host addresses, credentials or personal paths in it.

## 6. Checks, image and deployment

### Checks before a push

- Go: `go test ./...` (CI would use `go test --race ./...`) and `golangci-lint run ./...`.
- `PACK_LARGE=1 go test -run TestLargeFile ./pack/`: a sparse 4.7 GB file, about two minutes.
- pack's tests also check archives with `7z`, `unzip`, `python3` and `bsdtar` when they are on the
  PATH. On macOS, Homebrew's `p7zip` installs `7z`; the `sevenzip` formula installs `7zz`, which
  the tests do not look for. p7zip 17.05 fails `TestVolumes` ("Headers Error" on the
  `.zip.001` set) while 7-Zip 26.04 tests it clean, so put `7zz` first on the PATH as `7z`
  (`mkdir -p /tmp/bin7 && ln -sf /opt/homebrew/bin/7zz /tmp/bin7/7z`, then `PATH=/tmp/bin7:$PATH`).
- Frontend, in `frontend/`: `pnpm install --frozen-lockfile`, `npm run lint`,
  `npx vue-tsc --noEmit -p tsconfig.app.json`, `npx vitest run`, `pnpm run build`.
- The tests last ran on Linux. On macOS, APFS ignores letter case and refuses names that are not
  UTF-8; pack's tests allow for both, other packages' tests may not.
- Gezgin has no Playwright setup (upstream removed it); browser checks were ad-hoc Playwright
  scripts run against a local `go build` of Gezgin or, through a port forward, against nrm.

### Image and deployment

- A push to `main` builds `ghcr.io/drs0me1/gezgin:main` in about five minutes; the image config's
  `org.opencontainers.image.revision` label names the commit it was built from.
- On nrm, as root, through Konsol's container API on its Unix socket:

  ```sh
  # The Caddyfile names two panel.* hosts; the API answers (200, else 403) to the real one.
  api() { curl -sS --unix-socket /run/master-panel/api.sock -H "Host: panel.<domain>" -H "X-Konsol: 1" "$@"; }
  rev=$(api "http://panel.<domain>/api/konsol/konteynerler/ayrinti?ad=gezgin" |
    python3 -c 'import json,sys; print(json.load(sys.stdin)["revision"])')
  api -H "Content-Type: application/json" \
    --data-binary "{\"action\":\"image-update\",\"name\":\"gezgin\",\"revision\":\"$rev\"}" \
    "http://panel.<domain>/api/konsol/konteynerler/islem"
  # The job's state: /etc/master-stack/containers/operations/<id>.json
  podman inspect gezgin --format '{{index .Config.Labels "org.opencontainers.image.revision"}}'
  ```

- Gezgin listens on the tailnet on port 8091, its WebDAV shares on 8092; the container's `/srv` is
  the operator's media folder (`/srv/media` on the host). nrm runs the image of `2d157c4e`.
- Live checks through the operator's session: Claude opens `http://nrm:8091/login` in its
  built-in browser, the operator signs in there (Claude does not type a password on a host that
  is not local), and Claude runs the API checks with `fetch` from that page (its token is in
  `localStorage.jwt`) and the screen checks by hand. Nothing is created or deleted. Used for
  heading 8.
- Live checks without the operator's password: read the image id while the container runs
  (`podman inspect --type container gezgin --format '{{.Image}}'`; Konsol's stop removes the
  container), stop `gezgin` through Konsol's API, add a test admin with a random password kept in a
  root-only file (`podman run --rm --network none --user 1000:1000 -v gezgin-db:/database
  --entrypoint /bin/filebrowser <image id> -d /database/filebrowser.db users add <name> <password>
  --perm.admin`), start it again, and run the checks as root in its network namespace
  (`nsenter -t <pid> -n`) against `127.0.0.1:8080`, so that the password never leaves the host.
  Afterwards the test admin deletes itself (`DELETE /api/users/{id}` with `current_password`), which
  empties its trash too, and the password file goes.

## 7. Live checks used for archives

Through the API (`POST /api/archive`, then poll `GET /api/archive`):

- a ZIP of a folder: `<folder>.zip` beside it with the contents at its root; again: `(2)`;
- a tar.gz with `"volume":1048576`: `.001`, `.002`, `.003`; opening it from `.002` works;
- `GET /api/raw/<folder>?algo=zip&zone=Europe/Istanbul`: text deflated, `.mkv` stored, local
  times;
- a sparse 4.7 GB file: a ZIP64 entry that `7z t` and `unzip -t` pass; `7z l -slt` shows
  `Characteristics = Zip64 UT:M:1` and no `Zip64_ERROR` (Go's own ZIP writer fails this);
- in the browser: right-click → "Arşiv oluştur" with a custom 1 MB volume ends with "(3 parça)";
  the header button; the phone's "more" menu; Ctrl+S opens the download dialog.
