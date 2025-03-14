FROM golang:1.24.0-bullseye

# Install necessary packages
RUN apt-get update && apt-get install -y \
    git \
    curl \
    make \
    gcc \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/

# Install Helm
RUN curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Create a non-root user
ARG USERNAME=vscode
ARG USER_UID=1000
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME \
    && apt-get update \
    && apt-get install -y sudo \
    && echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Set up GOPATH and permissions
ENV GOPATH=/go
ENV PATH=$PATH:/go/bin
RUN mkdir -p /go/bin /go/src \
    && chown -R $USERNAME:$USERNAME /go

USER $USERNAME

# Install Go tools
RUN go install golang.org/x/tools/gopls@v0.14.2 \
    && go install github.com/go-delve/delve/cmd/dlv@v1.22.1 \
    && go install honnef.co/go/tools/cmd/staticcheck@v0.4.6 \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.55.2

WORKDIR /workspace
