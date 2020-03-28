ARG alpine=latest
FROM alpine:$alpine as base

RUN apk update && apk add --no-cache \
    ffmpeg-libs

# 82 is the standard uid/gid for "www-data" in Alpine
ARG uid=82
ARG gid=82

RUN addgroup -g $gid -S www-data \
    && adduser -u $uid -D -S -G www-data www-data

################################################################################

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
    && npm install \
    && npm run build

################################################################################

FROM base as prod

COPY --from=build /aurelius/cmd/aurelius /aurelius

EXPOSE 9090

USER www-data
WORKDIR /aurelius
ENTRYPOINT ["./aurelius", "-db", "/db"]
