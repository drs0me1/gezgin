## Multistage build: First stage fetches dependencies
FROM alpine:3.23 AS fetcher

# install and copy ca-certificates, mailcap and tini-static
RUN apk update && \
    apk --no-cache add ca-certificates mailcap tini-static

## Second stage: Use lightweight BusyBox image for final runtime environment
FROM busybox:1.37.0-musl

# Define non-root user UID and GID
ENV UID=1000
ENV GID=1000

# Create user group and user
RUN addgroup -g $GID user && \
    adduser -D -u $UID -G user user

# Copy binary, scripts, and configurations into image with proper ownership
COPY --chown=user:user filebrowser /bin/filebrowser
COPY --chown=user:user docker/common/ /
COPY --chown=user:user docker/alpine/ /
COPY --chown=user:user --from=fetcher /sbin/tini-static /bin/tini
COPY --from=fetcher /etc/ca-certificates.conf /etc/ca-certificates.conf
COPY --from=fetcher /etc/ca-certificates /etc/ca-certificates
COPY --from=fetcher /etc/mime.types /etc/mime.types
COPY --from=fetcher /etc/ssl /etc/ssl

# Create data directories and set ownership
RUN mkdir -p /config /database /srv && \
    chown -R user:user /config /database /srv

# Gezgin: keep generated thumbnails with the database, so that a folder's thumbnails are made once
ENV FB_CACHE_DIR=/database/cache

# Set the user, volumes and exposed ports
USER user

VOLUME /srv /config /database

# Gezgin listens on 8080 (WebDAV shares, when on, on a port of their own: FB_WEBDAV_PORT)
EXPOSE 8080

ENTRYPOINT [ "tini", "--", "/init.sh" ]
