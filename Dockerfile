# Use Golang image as the base
FROM golang:1.24-alpine

# Set working directory
WORKDIR /app

# Install necessary packages
RUN apk add --no-cache git

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the Go application
RUN go build -o main .

# Expose port 8080
EXPOSE 8123

# Command to run the executable
CMD ["./main"]