#!/usr/bin/env bash
# Gezgin's test environment on a Mac (K138-K141): packages Gezgin the way the image workflow does,
# for the Mac's own processor, and runs it in Podman the way the test host does: port 8091,
# WebDAV shares on 8092, the database and settings in the volumes gezgin-db and gezgin-config, and
# a folder of the Mac as /srv. Only this Mac reaches it (127.0.0.1).
#
#   scripts/test-env.sh build   build the frontend, the binary and the image localhost/gezgin:test
#   scripts/test-env.sh start   (re)create the container from that image and start it
#   scripts/test-env.sh stop    remove the container and stop Podman's virtual machine
#   scripts/test-env.sh logs    follow the container's log
#   scripts/test-env.sh reset   remove the container, its database and its settings (not the files)
#
# Commands run in order, e.g. `scripts/test-env.sh build start`. The files are in
# $GEZGIN_TEST_DIR/dosyalar (by default ~/GezginTest/dosyalar), with a few samples the first time.
# The first start makes the account `admin` with a random password, kept in
# $GEZGIN_TEST_DIR/admin-sifre.txt, which only the Mac's user can read.

set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
dir="${GEZGIN_TEST_DIR:-$HOME/GezginTest}"
image=localhost/gezgin:test
name=gezgin
arch="$(uname -m)"
[ "$arch" = x86_64 ] && arch=amd64

# Podman runs containers in a Linux virtual machine, which runs only while we test.
machine() {
  case "$(podman machine inspect --format '{{.State}}' 2>/dev/null || echo missing)" in
  running) ;;
  missing)
    podman machine init --cpus 4 --memory 4096 --disk-size 40
    podman machine start >/dev/null
    ;;
  *) podman machine start >/dev/null ;;
  esac
}

build() {
  local version ctx
  version="$(git -C "$repo" rev-parse --short=8 HEAD)"
  # Changes not yet committed show in the version Gezgin reports.
  [ -z "$(git -C "$repo" status --porcelain)" ] || version="$version-yerel"
  ctx="$repo/build/test-image"
  rm -rf "$ctx"
  mkdir -p "$ctx"

  (cd "$repo/frontend" && pnpm install --frozen-lockfile && pnpm run build)
  (cd "$repo" && CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -o "$ctx/filebrowser" \
    -ldflags "-s -w -X github.com/filebrowser/filebrowser/v2/version.Version=$version -X github.com/filebrowser/filebrowser/v2/version.CommitSHA=$version" .)
  cp "$repo/Dockerfile" "$ctx/"
  cp -R "$repo/docker" "$ctx/"

  machine
  podman build --platform "linux/$arch" --tag "$image" \
    --label org.opencontainers.image.title=Gezgin \
    --label org.opencontainers.image.revision="$(git -C "$repo" rev-parse HEAD)" \
    "$ctx"
  echo "Built Gezgin $version as $image"
}

samples() {
  local files="$dir/dosyalar"
  mkdir -p "$files/Belgeler" "$files/Resimler" "$files/Arşivler" "$files/Proje/kaynak" \
    "$files/Boş klasör"
  printf '# Notlar\n\nGezgin test ortamı: bu klasör Mac'\''teki test klasörüdür.\n' \
    >"$files/Belgeler/notlar.md"
  printf 'Elma\nArmut\nÇilek\nÜzüm\nŞeftali\n' >"$files/Belgeler/liste.txt"
  cp "$repo"/frontend/public/img/icons/*.png "$repo/frontend/public/img/logo.svg" \
    "$files/Resimler/"
  cp "$repo/README.md" "$files/Proje/OKU.md"
  cp "$repo/main.go" "$repo/docker/alpine/init.sh" "$files/Proje/kaynak/"
  (cd "$files" && zip -qr "Arşivler/proje.zip" Proje && tar -czf "Arşivler/belgeler.tar.gz" Belgeler)
}

start() {
  machine
  podman image exists "$image" || build
  mkdir -p "$dir/dosyalar"
  [ -n "$(ls -A "$dir/dosyalar")" ] || samples

  # The first start makes the database, with the account admin and a random password.
  local hash=""
  if ! podman volume exists gezgin-db; then
    local password
    password="$(openssl rand -hex 8)"
    (umask 077 && printf '%s\n' "$password" >"$dir/admin-sifre.txt")
    hash="$(podman run --rm --entrypoint /bin/filebrowser "$image" hash "$password")"
  fi

  podman rm --force "$name" >/dev/null 2>&1 || true
  # keep-id makes the container's user (1000) the Mac's user on the folder of files.
  podman run --detach --name "$name" --userns=keep-id:uid=1000,gid=1000 \
    --publish 127.0.0.1:8091:8080 --publish 127.0.0.1:8092:8092 \
    --env FB_WEBDAV_PORT=8092 --env FB_PASSWORD="$hash" \
    --volume gezgin-db:/database --volume gezgin-config:/config \
    --volume "$dir/dosyalar:/srv" \
    "$image" >/dev/null
  # Earlier builds of the image, now unused.
  podman image prune --force --filter label=org.opencontainers.image.title=Gezgin >/dev/null
  echo "Gezgin: http://127.0.0.1:8091 (admin; password in $dir/admin-sifre.txt)"
}

stop() {
  [ "$(podman machine inspect --format '{{.State}}' 2>/dev/null)" = running ] || return 0
  podman rm --force "$name" >/dev/null 2>&1 || true
  podman machine stop >/dev/null
}

logs() {
  machine
  podman logs --follow --tail 50 "$name"
}

reset() {
  machine
  podman rm --force "$name" >/dev/null 2>&1 || true
  podman volume rm --force gezgin-db gezgin-config >/dev/null
  rm -f "$dir/admin-sifre.txt"
}

[ $# -gt 0 ] || {
  sed -n '7,11s/^#   //p' "$0"
  exit 1
}
for command in "$@"; do
  case "$command" in
  build | start | stop | logs | reset) "$command" ;;
  *)
    echo "unknown command: $command" >&2
    exit 1
    ;;
  esac
done
