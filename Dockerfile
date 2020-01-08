FROM alpine as base

RUN apk update && apk add --no-cache \
    ffmpeg-libs

# 82 is the standard uid/gid for "www-data" in Alpine
RUN addgroup -g 82 -S www-data \
    && adduser -u 82 -D -S -G www-data www-data

###

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

###

FROM base as prod

COPY --from=build /aurelius/cmd/aurelius /aurelius

EXPOSE 9090

USER www-data
WORKDIR /aurelius
ENTRYPOINT ["./aurelius", "-db", "/db"]
