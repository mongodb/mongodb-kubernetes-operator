ARG builder_image
FROM ${builder_image} as builder

RUN mkdir /workspace
WORKDIR /workspace

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o version-upgrade-hook cmd/versionhook/main.go

FROM busybox

COPY --from=builder /workspace/version-upgrade-hook /version-upgrade-hook
