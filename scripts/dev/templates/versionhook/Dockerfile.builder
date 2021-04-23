FROM golang AS builder

ENV GO111MODULE=on
ENV GOFLAGS="-mod=vendor"
ENV GOPATH ""

COPY go.mod go.sum ./
RUN go mod download

ADD . .

RUN go mod vendor && \
    go build -o build/_output/version-upgrade-hook -mod=vendor github.com/mongodb/mongodb-kubernetes-operator/cmd/versionhook

FROM busybox

COPY --from=builder /go/build/_output/version-upgrade-hook /version-upgrade-hook
