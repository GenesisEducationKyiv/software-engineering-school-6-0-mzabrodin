# Stage 1: Build
FROM golang:1.26.2-alpine AS builder

WORKDIR /app

# Cache dependencies as a separate layer
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build binaries
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o subscription ./cmd/subscription && \
    CGO_ENABLED=0 GOOS=linux go build -o emailer ./cmd/emailer && \
    CGO_ENABLED=0 GOOS=linux go build -o scanner ./cmd/scanner

# Stage 2: Run
FROM alpine:3.23.3

WORKDIR /app

# ca-certificates: required for HTTPS requests to GitHub API and SMTP
# tzdata: required for correct timezone handling
RUN apk --no-cache add ca-certificates tzdata && \
    addgroup -S app && adduser -S -G app -h /app app

COPY --from=builder --chown=app:app /app/subscription .
COPY --from=builder --chown=app:app /app/emailer .
COPY --from=builder --chown=app:app /app/scanner .
COPY --from=builder --chown=app:app /app/migrations ./migrations

USER app

EXPOSE 8080

CMD ["./subscription"]