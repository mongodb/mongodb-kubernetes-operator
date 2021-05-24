FROM golang:1.15

ENV GO111MODULE=on

# install docker client
RUN apt-get update -y  && apt-get install -y apt-transport-https ca-certificates curl gnupg2 software-properties-common \
    && curl -fsSL https://download.docker.com/linux/debian/gpg | apt-key add - \
    && add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian $(lsb_release -cs) stable" \
    && apt-get update -y \
    && apt-get install -y docker-ce

# install kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl \
    && chmod +x ./kubectl \
    && mv ./kubectl /usr/local/bin

# install kustomize
ENV KUSTOMIZE_VER 2.0.0
RUN curl -LO https://github.com/kubernetes-sigs/kustomize/releases/download/v${KUSTOMIZE_VER}/kustomize_${KUSTOMIZE_VER}_linux_amd64  \
    && chmod +x ./kustomize_${KUSTOMIZE_VER}_linux_amd64 \
    && mv ./kustomize_${KUSTOMIZE_VER}_linux_amd64 /usr/local/bin/kustomize


# install controller-gen
RUN go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1 \
    && mv /go/bin/controller-gen /usr/local/bin/controller-gen

# install python
COPY requirements.txt requirements.txt
RUN apt-get install -y  python3-pip && pip3 install --upgrade pip \
    && pip install -r requirements.txt \
    && ln -sf /usr/bin/python3 /usr/bin/python

# Note: must run `pip install git+ssh://git@github.com/10gen/sonar.git@0.0.10` manually once
# connected to container with correct permissions.

# install jq to parse json within bash scripts
RUN curl -o /usr/local/bin/jq http://stedolan.github.io/jq/download/linux64/jq && \
  chmod +x /usr/local/bin/jq


ENV OPERATOR_SDK_VERSION=1.3.0
RUN curl -L https://github.com/operator-framework/operator-sdk/releases/download/v${OPERATOR_SDK_VERSION}/operator-sdk_linux_amd64 -o /usr/local/bin/operator-sdk  \
    && chmod +x /usr/local/bin/operator-sdk


WORKDIR /workspace
