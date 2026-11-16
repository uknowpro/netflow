FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /netflow ./cmd/netflow

FROM alpine:3.23
RUN apk add --no-cache ca-certificates
COPY --from=build /netflow /usr/local/bin/netflow
USER 65532:65532
ENTRYPOINT ["netflow"]
CMD ["--help"]
