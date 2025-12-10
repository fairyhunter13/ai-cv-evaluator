# Build stage
FROM golang:1.25.5-alpine AS builder

WORKDIR /app

# Copy generic go.mod and go.sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o meta-exporter .

# Run stage
FROM python:3.11-alpine 
# Note: Using python:3.11-alpine just to match previous base size roughly, 
# or strictly stick to alpine/distroless. 
# Using alpine:latest for minimal footprint.
FROM alpine:latest

WORKDIR /app

# Install CA certificates for HTTPS if needed (though we talk to socket)
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/meta-exporter .

CMD ["./meta-exporter"]
