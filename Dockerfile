# Mount points:
# - /media: Media library. The path can be overridden with the MEDIA_PATH build argument.
# - /storage: Persistent storage (favorites, etc.). A volume will be created if it is not mounted.

# Shared base stage ############################################################

ARG ALPINE_VER=latest
FROM alpine:$ALPINE_VER AS base

RUN apk update && apk add --no-cache \
    ffmpeg-libs \
    musl-dev

# 82 is the standard uid/gid for "www-data" in Alpine
ARG UID=82

RUN adduser -u $UID -D -S -G www-data www-data

# Build stage ##################################################################

FROM base AS build

RUN apk add --no-cache \
    ffmpeg-dev \
    gcc \
    go \
    npm

COPY --chown=www-data:www-data . /aurelius

USER www-data
WORKDIR /aurelius/cmd/aurelius
RUN go build \
    && npm install --only=prod \
    && npm run build

# Production stage #############################################################

FROM base AS prod

COPY --from=build /aurelius/cmd/aurelius /aurelius
COPY docker/entrypoint.sh /aurelius

EXPOSE 9090

ARG MEDIA_PATH=/media
ENV MEDIA_PATH=$MEDIA_PATH

RUN mkdir /storage
RUN chown www-data:www-data /storage
VOLUME ["/storage"]

USER www-data
WORKDIR /aurelius
ENTRYPOINT ["/bin/sh", "entrypoint.sh"]
