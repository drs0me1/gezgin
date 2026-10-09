# Handover — 2026-10-09

The work moves from a cloud session to a local session on the operator's MacBook. This file is the
to-do list: tick items off, add new ones, and delete the file once it is empty. Run `git pull`
first. Speak Turkish to the operator; code comments, docs and commit messages stay in English.

## Where things stand

- `main`: this commit, on top of
  - `601ddbaa` making ZIP, tar and tar.gz archives on the server (K54–K58);
  - `6d531015` files only named like RAR parts stay files (K53);
  - `527111be`, `b0bb8b63` opening archives on the server, in Go (K47–K51);
  - `116c3549` command runner and file-event hooks removed (K45–K46).
- The test host **nrm** runs the image built from `601ddbaa` (updated through Konsol), verified
  live; see "Live checks" below. Its admin password is the operator's test password, not written
  here.
- Decisions are numbered K1–K58 so far; the next one is **K59**. Each decision's outcome is in
  its commit message and in README's "Changes from File Browser".
- The myserver repository has nothing pending from this work.
- CI (`.github/workflows/ci.yaml`) runs on `master` and pull requests only, so a push to `main`
  runs no tests, only the image build (`gezgin-image.yml`). Run the checks below before pushing.

## To do

### 1. Archive creation follow-ups (commit `601ddbaa`)

- [ ] **C1 control characters.** `pack.archiveName` refuses only runes below 0x20 and 0x7f, while
  `unpack.clean` refuses every `unicode.IsControl` rune (also U+0080–U+009F). A file with such a
  name goes into the archive, and Gezgin's own "Arşivi aç" then refuses the whole archive
  (`unsafe`). Use `unicode.IsControl` in `pack.archiveName` and in `http.archiveName`; add tests.
- [ ] **Depth of files.** `pack`'s `walk` checks `maxDepth` (64 parts) only for folders. A file in
  a folder 64 deep gets 65 parts and `unpack` refuses the archive. Check the depth in `add()` for
  every entry; add a test.
- [ ] **Chosen `.gezgin-` items.** Names starting `.gezgin-` are skipped inside folders, but an item
  the user chooses directly (an upload's `.gezgin-*.tmp`, which listings show) is packed, and
  `unpack` refuses that name. Refuse the prefix in `pack.archiveName` (skipped and counted).
- [ ] **Independent review** of `git diff 6d531015 601ddbaa`: a review workflow was stopped before
  it reported. Dimensions:
  - ZIP/tar/volume writers (`pack/zip.go`, `tar.go`, `output.go`): APPNOTE fields and offsets,
    ZIP64 extras (local: both sizes; central: only the fields that are 0xFFFFFFFF), end records,
    stream-mode data descriptors, patching across volumes, byte and volume limits;
  - packing safety (`pack/pack.go`, `http/archive.go` `packSource`, `http/raw.go`): links and the
    reserved real paths, loops, special files, change detection (`os.SameFile` and ctime), rules,
    names `unpack` takes back; downloads have no entry limit (memory on huge trees);
  - HTTP jobs and UI (`http/archive.go`, `Archive.vue`, `ArchiveJobs.vue`, i18n): validation,
    permissions, `publishFiles` numbering, stale volumes and rollback, the job JSON;
  - unpack (`unpack/names.go`, `formats.go`): K53's RAR part logic, volume sets, the joined
    reader, which errors become `missingPart`.
- [ ] Redeploy to nrm after the fixes and repeat the live checks.
- [ ] On nrm, delete the test folder `/k54-deneme` permanently. It holds a sparse 4.7 GB
  `buyuk/buyuk.bin` (it takes no space, but a download would send 4.7 GB), ZIPs and volume sets.
- [ ] Try on real clients (not done yet): Windows 11 Explorer (a ZIP with Turkish names; times
  should show in local time), macOS Archive Utility, 7-Zip on Windows with a `.zip.001` set, an
  entry over 4 GiB on Windows.

### 2. Seen, not yet decided (bring to the operator)

- [ ] Listings type `.001`, `.r00`, `.tgz` and similar files as plain files (icons) and small
  archives as text (part of heading 9).
- [ ] Unknown `/api/*` paths answer 200 with the app's `index.html` instead of 404 (heading 9).
- [ ] Downloads have no entry limit and plan every entry in memory first, as File Browser did.
- [ ] Should CI run on `main` instead of `master`?

### 3. Next headings, in the operator's order

- [ ] **8 — Admin settings screen.** Remove the registration settings and any leftovers of the
  command settings; branding becomes a fixed Gezgin brand without custom CSS; keep default user
  settings, rules, upload settings, the minimum password length and the permission modes.
- [ ] **9 — Infrastructure, branding, CSP.** A CSP for the index; the inline startup script moved
  to a file; Gezgin's name, logo, icons and PWA manifest; remove "File Browser (untracked)", the
  "Sorun bildir" and other upstream links, and the "project archived" log lines; review the
  Turkish texts.
- [ ] **Taken from Konsol's Files module.** Folder sizes and item counts, favorites (archive
  extraction is done).
- [ ] Later: K52, retiring Konsol's own ZIP/RAR (myserver repository); video transcoding; WebDAV
  through Konsol's HTTPS; perhaps retiring Konsol's WebDAV.

## How to work

- Commit and push to `main` unless the operator says otherwise.
- Per heading: read the endpoints and the code, check on nrm, report in Turkish with numbered
  decisions and a recommendation, implement once approved, test, commit, push, wait for the image,
  update nrm, verify live.
- nrm is a disposable test host: data loss is acceptable, make no server-side backups, and test
  with the main `gezgin` container instead of throwaway ones.

### Checks before a push

- Go: `go test ./...` (CI would use `go test --race ./...`) and `golangci-lint run ./...`.
- `PACK_LARGE=1 go test -run TestLargeFile ./pack/`: a sparse 4.7 GB file, about two minutes.
- pack's tests also check archives with `7z`, `unzip`, `python3` and `bsdtar` when they are on the
  PATH. On macOS, Homebrew's `p7zip` installs `7z`; the `sevenzip` formula installs `7zz`, which
  the tests do not look for.
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
  the operator's media folder.

### Live checks used for archives

Through the API (`POST /api/archive`, then poll `GET /api/archive`):

- a ZIP of a folder: `<folder>.zip` beside it with the contents at its root; again: `(2)`;
- a tar.gz with `"volume":1048576`: `.001`, `.002`, `.003`; opening it from `.002` works;
- `GET /api/raw/<folder>?algo=zip&zone=Europe/Istanbul`: text deflated, `.mkv` stored, local
  times;
- a sparse 4.7 GB file: a ZIP64 entry that `7z t` and `unzip -t` pass; `7z l -slt` shows
  `Characteristics = Zip64 UT:M:1` and no `Zip64_ERROR` (Go's own ZIP writer fails this);
- in the browser: right-click → "Arşiv oluştur" with a custom 1 MB volume ends with "(3 parça)";
  the header button; the phone's "more" menu; Ctrl+S opens the download dialog.
