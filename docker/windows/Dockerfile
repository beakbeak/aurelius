# This Dockerfile produces a Windows binary package of Aurelius using MinGW.
# The Aurelius Git repository should be mounted at /src
# The package will be built as /src/aurelius.zip

FROM fedora:31

ARG FFMPEG_VER=4.2.2
ARG FFMPEG_SHA256=b620d187c26f76ca19e74210a0336c3b8380b97730df5cdf45f3e69e89000e5c

# LAME 3.100 fails to compile on MinGW: https://sourceforge.net/p/lame/bugs/487/
ARG LAME_VER=3.99.5
ARG LAME_SHA256=24346b4158e4af3bd9f2e194bb23eb473c75fb7377011523353196b19b9a23ff

RUN yum update -y && yum install -y \
        bzip2 \
        diffutils \
        golang \
        make \
        mingw64-gcc \
        mingw64-libvorbis \
        mingw64-pkg-config \
        nasm \
        npm \
        yasm

WORKDIR /build

RUN curl -OL https://downloads.sourceforge.net/lame/lame-${LAME_VER}.tar.gz \
    && sha256sum lame-${LAME_VER}.tar.gz \
    && echo "${LAME_SHA256}  lame-${LAME_VER}.tar.gz" | sha256sum -c \
    && tar xvf lame-${LAME_VER}.tar.gz

WORKDIR lame-${LAME_VER}
# --prefix doesn't seem to work with mingw64-configure
RUN mingw64-configure && make && make install
WORKDIR ..

RUN curl -OL https://ffmpeg.org/releases/ffmpeg-${FFMPEG_VER}.tar.bz2 \
    && sha256sum ffmpeg-${FFMPEG_VER}.tar.bz2 \
    && echo "${FFMPEG_SHA256}  ffmpeg-${FFMPEG_VER}.tar.bz2" | sha256sum -c \
    && tar xvf ffmpeg-${FFMPEG_VER}.tar.bz2

WORKDIR ffmpeg-${FFMPEG_VER}
RUN ./configure \
        --enable-libvorbis --enable-libmp3lame \
        --enable-shared \
        --enable-cross-compile --arch=x86_64 --target-os=mingw32 --cross-prefix=x86_64-w64-mingw32- \
        --prefix=/usr/x86_64-w64-mingw32/sys-root/mingw/local \
    && make && make install
WORKDIR ..

COPY entrypoint.sh .
CMD ["/bin/bash", "entrypoint.sh"]
