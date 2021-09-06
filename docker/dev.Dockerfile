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

ARG GO_VERSION=1.17

RUN wget https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz \
    && rm -f go${GO_VERSION}.linux-amd64.tar.gz

RUN groupadd -g 1000 code \
    && useradd -g 1000 -u 1000 -m -s /usr/bin/bash code

# allow default user to run sudo without a password
RUN mkdir -p /etc/sudoers.d && \
    echo 'code ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/90-code-nopasswd

USER code

RUN echo "export PATH=${PATH}:/usr/local/go/bin:~/go/bin" >> /home/code/.profile

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/77e211ba75b7802fe5e40b276bca0e928553fc7f/install.sh \
    | sh -s -- -b $(go env GOPATH)/bin v1.24.0

EXPOSE 9090
