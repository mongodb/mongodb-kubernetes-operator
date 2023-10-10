ARG builder_image
FROM --platform=$BUILDPLATFORM ${builder_image} as builder

RUN mkdir /workspace
WORKDIR /workspace

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o version-upgrade-hook cmd/versionhook/main.go

FROM busybox

COPY --from=builder /workspace/version-upgrade-hook /version-upgrade-hook
