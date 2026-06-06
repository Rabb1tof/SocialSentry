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
COPY go.mod ./
COPY go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
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
