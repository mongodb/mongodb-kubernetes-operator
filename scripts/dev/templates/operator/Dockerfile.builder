ARG builder_image
FROM ${builder_image} AS builder

RUN mkdir /workspace
WORKDIR /workspace

COPY go.mod go.sum ./
COPY cmd/manager/main.go cmd/manager/main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/
COPY build/bin/ build/bin/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager cmd/manager/main.go

FROM busybox

RUN mkdir /workspace
COPY --from=builder /workspace/manager /workspace/
COPY --from=builder /workspace/build/bin /workspace/build/bin
