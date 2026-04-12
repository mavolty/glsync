# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o bin/glsync ./cmd/glsync

# Runtime stage — minimal image
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/bin/glsync /glsync
COPY --from=builder /app/config /config
COPY --from=builder /app/migrations /migrations

EXPOSE 8090
ENTRYPOINT ["/glsync"]
CMD ["-config", "/config/glsync.yaml"]
