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

Details: [docs/authentication.md](docs/authentication.md).

The upstream README is kept in the history (`git show 833d9088:README.md`).

## License

Apache License 2.0, as the original. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
