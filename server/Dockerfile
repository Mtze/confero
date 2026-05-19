FROM golang:1.24 AS builder
WORKDIR /src

# Dependencies first for layer caching.
COPY server/go.mod server/go.sum ./
RUN go mod download

# Source
COPY server/ ./
COPY api/ ../api/

ARG VERSION=dev
ARG COMMIT=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w \
      -X confero/internal/version.Version=${VERSION} \
      -X confero/internal/version.Commit=${COMMIT}" \
    -o /out/confero-server ./cmd/confero-server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/confero-server /confero-server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/confero-server"]
