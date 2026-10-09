# CLAUDE.md

Guidance for Claude in this repository: Gezgin, a file manager for the Konsol server panel and a
modified fork of File Browser (archived upstream).

- Read `HANDOVER.md` first, top to bottom. It holds the stages of the work, every decision taken
  (K1, K2, ...), the open tasks, the checks and the deployment steps. Keep it current: tick tasks
  off, record decisions as the operator takes them.
- Speak Turkish to the operator. Code comments, docs and commit messages are in English.
- Work heading by heading: review the code and endpoints, report the verified problems in Turkish
  with numbered decisions and a recommendation, implement once the operator decides, test, package
  and run it on the Mac (`scripts/test-env.sh build start`), check it there and commit to `main`.
  Push only when the operator says so ("push et"); then wait for the image, update the test host
  and verify live.
- Keep changes small and match the surrounding code. The Go module path stays
  `github.com/filebrowser/filebrowser/v2` and the program is still `filebrowser`.
- The repository is public: no host addresses, credentials or personal paths in it.
