# This Dockerfile is for development purposes (via Visual Studio Code) only.
# It is referenced by .devcontainer/devcontainer.json

FROM archlinux

RUN pacman -Sy && pacman -S --noconfirm \
    ffmpeg \
    gcc \
    git \
    go \
    npm \
    pkgconf \
    sudo \
    vim

RUN groupadd -g 1000 code \
    && useradd -g 1000 -u 1000 -m code

# allow default user to run sudo without a password
RUN mkdir -p /etc/sudoers.d && \
    echo 'code ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/90-code-nopasswd

USER code

RUN echo "export PATH=${PATH}:~/go/bin" >> /home/code/.bashrc && \
    GO111MODULE=on go get -v \
    golang.org/x/tools/gopls@latest \
    github.com/ramya-rao-a/go-outline \
    github.com/go-delve/delve/cmd/dlv

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/77e211ba75b7802fe5e40b276bca0e928553fc7f/install.sh \
    | sh -s -- -b $(go env GOPATH)/bin v1.24.0

EXPOSE 9090
