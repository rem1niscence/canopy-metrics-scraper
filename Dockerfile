# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies for CGO
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go module files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o metrics-scraper ./cmd

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/metrics-scraper .

# Create directory for the database
RUN mkdir -p /app/data

# Command to run the executable with parameters that can be overridden
ENTRYPOINT ["./metrics-scraper"]
CMD ["--dbName", "/app/data/metrics.sqlite3", "--url", "http://localhost:50002", "--metricsURL", "http://localhost:9090/metrics", "--chainID", "1"]
