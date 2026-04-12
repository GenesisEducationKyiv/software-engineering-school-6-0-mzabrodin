# Stage 1: Build
FROM golang:1.26.2-alpine AS builder

WORKDIR /app

# Cache dependencies as a separate layer
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Stage 2: Run
FROM alpine:3.23.3

WORKDIR /app

# ca-certificates: required for HTTPS requests to GitHub API and SMTP
# tzdata: required for correct timezone handling
RUN apk --no-cache add ca-certificates tzdata

# Copy only necessary artifacts from builder stage
COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./server"]