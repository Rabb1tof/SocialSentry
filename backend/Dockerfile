# Multi-stage Dockerfile for the SocialSentry backend (api + worker)

# === Development stage (used by docker-compose for local dev) ===
FROM golang:1.25-alpine AS development
WORKDIR /app
RUN apk add --no-cache git
# Install goose for the migrate service. Lives in /go/bin which isn't shadowed by
# the host bind mount on /app.
RUN go install github.com/pressly/goose/v3/cmd/goose@latest
COPY go.mod ./
COPY go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go mod tidy
EXPOSE 8080
CMD ["go", "run", "./cmd/api"]

# === Builder stage (used by production image) ===
FROM golang:1.25-alpine AS builder
WORKDIR /app
ARG TARGETARCH=amd64
RUN apk add --no-cache curl
COPY go.mod ./
COPY go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
# Regenerate the gitignored sqlc code (internal/db/generated) so the image builds
# from a clean checkout. Use the prebuilt sqlc binary (not `go run`) so sqlc is
# decoupled from the image's Go version. Keep SQLC_VERSION in sync with ci.yml.
RUN curl -fsSL "https://downloads.sqlc.dev/sqlc_1.31.1_linux_${TARGETARCH}.tar.gz" -o /tmp/sqlc.tar.gz \
    && tar -xzf /tmp/sqlc.tar.gz -C /usr/local/bin sqlc \
    && rm /tmp/sqlc.tar.gz \
    && sqlc generate
RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /worker ./cmd/worker

# === Production stage: API (default command) ===
# Both binaries live in the image; the default command runs the HTTP API.
FROM alpine:3.19 AS production
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /api /worker /usr/local/bin/
EXPOSE 8080
CMD ["api"]

# === Production stage: worker ===
# Same image as `production`, but the bot engine is the default command.
# Built as a separate target so api/worker ship as distinct runnable images.
FROM production AS production-worker
CMD ["worker"]

# === Migration runner image ===
# Ships goose + the migration SQL so production (which pulls images, not source)
# can apply migrations. Used as a one-shot service in docker-compose.prod.yml.
FROM golang:1.25-alpine AS goose-build
RUN go install github.com/pressly/goose/v3/cmd/goose@latest

FROM alpine:3.19 AS migrate
RUN apk add --no-cache ca-certificates
COPY --from=goose-build /go/bin/goose /usr/local/bin/goose
COPY internal/db/migrations /migrations
ENV GOOSE_DRIVER=postgres \
    GOOSE_MIGRATION_DIR=/migrations
# Pass the DSN via GOOSE_DBSTRING at runtime; defaults to `goose up`.
ENTRYPOINT ["goose"]
CMD ["up"]
