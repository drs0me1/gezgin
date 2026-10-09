# Gezgin documentation

Gezgin is a file manager for the Konsol server panel, a modified fork of File Browser. The
[README](../README.md) sums up what it changes from File Browser; [changes.md](changes.md) lists
every change in detail.

## Contents

- [Changes from File Browser, in detail](changes.md)
- [Authentication](authentication.md)
- [Customization](customization.md)
- [Troubleshooting](troubleshooting.md)
- [Command Line Usage](cli/filebrowser.md)

## Running

Gezgin runs as a container, `ghcr.io/drs0me1/gezgin:main`, which a push to `main` builds. It
listens on port 8080 (`FB_PORT`), serves the files of `/srv`, keeps its database and thumbnails in
`/database` and its configuration in `/config`. WebDAV shares are served on a port of their own
when `FB_WEBDAV_PORT` is set. On first start with an empty database, quick setup creates the admin
and logs a generated password, to be changed at the first login.

## Building

```sh
cd frontend && pnpm install --frozen-lockfile && pnpm run build && cd ..
go build -o filebrowser .
```

The program is still called `filebrowser` and the Go module path stays
`github.com/filebrowser/filebrowser/v2`.
