ARG builder_image
FROM ${builder_image} AS builder

RUN mkdir /workspace
WORKDIR /workspace

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager cmd/manager/main.go

FROM busybox

RUN mkdir /workspace
COPY --from=builder /workspace/manager /workspace/
COPY --from=builder /workspace/build/bin /workspace/build/bin
