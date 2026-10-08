# Gezgin

Gezgin is a file manager for the [Konsol](https://github.com/drs0me1/myserver) server panel. It is a
modified fork of [File Browser](https://github.com/filebrowser/filebrowser) (Apache-2.0), which was
archived on 2026-09-01. The original history and licence are kept; see [NOTICE](NOTICE) and
[LICENSE](LICENSE).

## Status

Work in progress. Until the first Gezgin release this tree is File Browser's last state
(`v2.63.23`, commit `833d9088`) plus the changes listed in the commit history.

## Direction

- Runs as a Podman container behind Konsol's Caddy and session, on the tailnet only.
- Konsol signs the operator in; Gezgin keeps no accounts or passwords of its own.
- Parts that Konsol does not use or that do not fit it are removed (for example the command
  runner, sign-up and the user administration screens).
- Everything the page loads comes from its own origin (no CDN), so it works under Konsol's
  Content-Security-Policy and without internet access on the tailnet.
- Its own name and look; no File Browser branding in Gezgin builds.

The upstream README is kept in the history (`git show 833d9088:README.md`).

## License

Apache License 2.0, as the original. See [LICENSE](LICENSE) and [NOTICE](NOTICE).
