FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /netflow ./cmd/netflow

# Install the architecture-independent trust bundle on the builder platform.
FROM --platform=$BUILDPLATFORM alpine:3.23 AS certificates
RUN apk add --no-cache ca-certificates

FROM alpine:3.23
COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /netflow /usr/local/bin/netflow
USER 65532:65532
ENTRYPOINT ["netflow"]
CMD ["--help"]
