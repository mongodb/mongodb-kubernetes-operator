FROM golang:1.18.5-alpine as builder

COPY ./cmd/readiness /build/
COPY ./pkg /build/pkg
COPY ./api /build/api
COPY ./go.mod /build/
COPY ./go.sum /build/

WORKDIR /build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -i -o readinessprobe .

FROM busybox

COPY --from=builder /build/readinessprobe /probes/
