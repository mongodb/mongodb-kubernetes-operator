ARG builder_image
FROM --platform=$BUILDPLATFORM ${builder_image} AS builder

RUN mkdir /workspace
WORKDIR /workspace

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o manager cmd/manager/main.go

FROM busybox

RUN mkdir /workspace
COPY --from=builder /workspace/manager /workspace/
COPY --from=builder /workspace/build/bin /workspace/build/bin
