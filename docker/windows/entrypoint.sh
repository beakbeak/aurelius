#!/bin/bash

set -xe

git clone --depth 1 file:///src aurelius
cd aurelius/cmd/aurelius

npm install
npm run build

CC=x86_64-w64-mingw32-gcc \
    PKG_CONFIG=mingw64-pkg-config \
    PKG_CONFIG_PATH=/usr/x86_64-w64-mingw32/sys-root/mingw/local/lib/pkgconfig \
    GOOS=windows \
    GOARCH=amd64 \
    CGO_ENABLED=1 \
    GO111MODULE=on \
    go build -v

cp \
    /usr/x86_64-w64-mingw32/sys-root/mingw/bin/libvorbis*.dll \
    /usr/x86_64-w64-mingw32/sys-root/mingw/bin/libogg*.dll \
    /usr/x86_64-w64-mingw32/sys-root/mingw/bin/libmp3lame*.dll \
    /usr/x86_64-w64-mingw32/sys-root/mingw/local/bin/*.dll \
    .

cd ..
rm -f /src/aurelius.zip
zip -9r /src/aurelius.zip aurelius -x '*.go' .gitignore
