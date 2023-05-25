# Stage 1: Build
FROM golang:1.20 AS builder

# Copy the source code
COPY ./app /app

# Set the working directory
WORKDIR /app

# Download Go modules
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /alert-management


# Stage 2: Final image
FROM alpine:latest

# Copy the built binary from the previous stage
COPY --from=builder /alert-management /alert-management
COPY --from=builder /app/configs/config.yaml /configs/config.yaml

# Optional: Set the default port the application listens on
EXPOSE 8080

# Set the entrypoint command to run the application
CMD ["/alert-management"]