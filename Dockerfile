# Build Stage
FROM golang:1.25-alpine AS builder

# Install git for fetching dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o main ./cmd/api/main.go

# Runtime Stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/main .

# Expose the application port
EXPOSE 5000

# Command to run the executable
CMD ["./main"]
