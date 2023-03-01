# This Dockerfile is for development purposes (via Visual Studio Code) only.
# It is referenced by .devcontainer/devcontainer.json

FROM ubuntu:latest

RUN apt-get update && DEBIAN_FRONTEND="noninteractive" apt-get install -y \
    ffmpeg \
    gcc \
    git \
    libavformat-dev \
    npm \
    pkg-config \
    sudo \
    wget

ARG GO_VERSION=1.20.1

RUN arch=$(arch | sed -e s/aarch64/arm64/ -e s/x86_64/amd64/) \
    && wget https://golang.org/dl/go${GO_VERSION}.linux-${arch}.tar.gz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf go${GO_VERSION}.linux-${arch}.tar.gz \
    && rm -f go${GO_VERSION}.linux-${arch}.tar.gz

RUN groupadd -g 1000 code \
    && useradd -g 1000 -u 1000 -m -s /usr/bin/bash code

# allow default user to run sudo without a password
RUN mkdir -p /etc/sudoers.d && \
    echo 'code ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/90-code-nopasswd

USER code

RUN echo "export PATH=${PATH}:/usr/local/go/bin:~/go/bin" >> /home/code/.profile

EXPOSE 9090
