# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY migrations/ ./migrations/

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/me-lab-api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata curl sqlite \
    && addgroup -S -g 10001 melab \
    && adduser -S -u 10001 -G melab melab \
    && mkdir -p /data \
    && chown -R melab:melab /data

WORKDIR /app
COPY --from=builder /out/me-lab-api /app/me-lab-api

USER melab:melab
EXPOSE 8080

ENV PORT=8080 \
    DATABASE_URL=/data/me-lab.db \
    LOG_LEVEL=info

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/app/me-lab-api"]
