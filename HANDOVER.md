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
remain before heading 7 is closed.

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
| 8 | Yönetim ayarları ekranı | from K66 | | To do (4.3) |
| 9 | Altyapı, marka ve CSP | | | To do (4.3) |
| 10 | Konsol'dan alınacaklar | | | To do (4.3); archives already done under 7 |

Commit messages and README's "Changes from File Browser" describe each change.

## 3. Decisions taken

The next decision number is **K66**. "Recommended" means the operator accepted the recommendation
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

- [ ] **8 — Yönetim ayarları ekranı.** The opening overview proposed: remove the registration
  settings (decided: K60, in 4.6) and any leftovers of the
  command settings (gone with K46); branding becomes a fixed Gezgin brand without custom CSS; keep
  the default user settings, rules, upload settings, the minimum password length and the
  permission modes.
- [ ] **9 — Altyapı, marka ve CSP.** Proposed: a CSP for the index page; the inline startup
  script moved to a file; Gezgin's name, logo, icons and PWA manifest; remove
  "File Browser (untracked)", the "Sorun bildir" and other upstream links and the "project
  archived" log lines; review the Turkish texts. Keep: one Go program with the interface built in,
  BoltDB, the CLI (`users` resets an admin password), `/health`, the 34 languages with Turkish by
  default, the local Material Icons and Roboto fonts; File Browser's own TLS and Unix socket stay
  off (Caddy is in front). Already done: Gezgin's own image on port 8080. Seen since, for this
  heading: listings type `.001`, `.r00`, `.tgz` and similar as plain files and small archives as
  text; unknown `/api/*` paths answer 200 with `index.html` instead of 404.
- [ ] **10 — Konsol'dan alınacaklar.** Folder sizes and item counts on folder tiles (Konsol
  DD-247/248) and favourite folders (DD-250). Archive extraction and creation are done (7).

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
- [ ] Later cleanup: `settings.CreateUserHome`, `users.Storage.SaveProvisioned` and `GetByScope`
  served only signup and proxy sign-in and are unused now.

## 5. How to work

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
  the operator's media folder (`/srv/media` on the host). nrm runs the image of `45c22b38`.
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
