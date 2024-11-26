# Stage 1: Build the Go application
FROM golang:1.23 as builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum from the parent directory
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the entire source code including the extensions directory
COPY extensions ./extensions

# Build the application
WORKDIR /app/extensions
RUN CGO_ENABLED=0 GOOS=linux go build -o /main .

# Stage 2: Create a minimal runtime image
FROM alpine:3.20

# Install necessary certificates for HTTPS
RUN apk add --no-cache ca-certificates

# Set the working directory
WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /main .

# Expose the application port
EXPOSE 8080

# Command to run the application
CMD ["./main"]
