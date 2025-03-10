# Mount points:
# - /media: Media library. The path can be overridden with the MEDIA_PATH build argument.
# - /storage: Persistent storage (favorites, etc.). A volume will be created if it is not mounted.

# Shared base stage ############################################################

ARG ALPINE_VER=latest
FROM alpine:$ALPINE_VER AS base

RUN apk update && apk add --no-cache \
    ffmpeg-libs \
    musl-dev

ARG GID=1001
ARG UID=1001

RUN addgroup -g $GID -S aurelius \
    && adduser -u $UID -D -S -G aurelius aurelius

# Build stage ##################################################################

FROM base AS build

RUN apk add --no-cache \
    ffmpeg-dev \
    gcc \
    go \
    npm

COPY --chown=aurelius:aurelius . /aurelius

USER aurelius
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
RUN chown aurelius:aurelius /storage
VOLUME ["/storage"]

USER aurelius
WORKDIR /aurelius
ENTRYPOINT ["/bin/sh", "entrypoint.sh"]
