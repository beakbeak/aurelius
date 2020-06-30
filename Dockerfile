# Media library should be mounted at /media

# Shared base stage ############################################################

ARG ALPINE_VER=latest
FROM alpine:$ALPINE_VER as base

RUN apk update && apk add --no-cache \
    ffmpeg-libs

# 82 is the standard uid/gid for "www-data" in Alpine
ARG UID=82
ARG GID=82

RUN addgroup -g $GID -S www-data \
    && adduser -u $UID -D -S -G www-data www-data

# Build stage ##################################################################

FROM base as build

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

FROM base as prod

COPY --from=build /aurelius/cmd/aurelius /aurelius

EXPOSE 9090

USER www-data
WORKDIR /aurelius
ENTRYPOINT ["./aurelius", "-media", "/media"]
