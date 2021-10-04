# [Choice] Go version: 1, 1.16, 1.15
ARG VARIANT=1.17
FROM mcr.microsoft.com/vscode/devcontainers/go:0-${VARIANT}

# install mage
RUN git clone https://github.com/magefile/mage && \
    cd mage && \
    go run bootstrap.go && \
    cd .. && \
    rm -rf mage

# install helm
RUN curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash

# install k9s
RUN curl -L -o k9s.tgz https://github.com/derailed/k9s/releases/download/v0.24.15/k9s_Linux_x86_64.tar.gz && \
    tar -xzf k9s.tgz && \
    mv k9s /usr/local/bin/k9s && \
    rm k9s.tgz

# install kubebuilder / controller-gen
RUN curl -L -o kubebuilder https://go.kubebuilder.io/dl/latest/$(go env GOOS)/$(go env GOARCH) && \
    chmod +x kubebuilder && mv kubebuilder /usr/local/bin/

# install kubectl
RUN apt-get update && \
    apt-get install -y apt-transport-https ca-certificates curl && \
    sudo curl -fsSLo /usr/share/keyrings/kubernetes-archive-keyring.gpg https://packages.cloud.google.com/apt/doc/apt-key.gpg && \
    echo "deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list && \
    sudo apt-get update && \ 
    sudo apt-get install -y kubectl

# [Optional] Uncomment this section to install additional OS packages.
# RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
#     && apt-get -y install --no-install-recommends <your-package-list-here>

# [Optional] Uncomment the next line to use go get to install anything else you need
# RUN go get -x <your-dependency-or-tool>

# [Optional] Uncomment this line to install global node packages.
# RUN su vscode -c "source /usr/local/share/nvm/nvm.sh && npm install -g <your-package-here>" 2>&1
