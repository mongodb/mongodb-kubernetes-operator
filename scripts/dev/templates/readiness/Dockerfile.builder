ARG builder_image
FROM --platform=$BUILDPLATFORM ${builder_image} as builder

RUN mkdir /workspace
WORKDIR /workspace

COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o readinessprobe cmd/readiness/main.go

FROM busybox

COPY --from=builder /workspace/readinessprobe /probes/
