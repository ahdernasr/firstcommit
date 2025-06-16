# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Python build stage
FROM python:3.11-slim-bullseye AS python-builder

# Set working directory
WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Create and activate virtual environment
RUN python3 -m venv /app/venv
ENV PATH="/app/venv/bin:$PATH"

# Install pip tools
RUN pip install --no-cache-dir --upgrade pip setuptools wheel

# Copy requirements
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Final stage
FROM python:3.11-slim-bullseye

# Set working directory
WORKDIR /app

# Copy virtual environment from builder
COPY --from=python-builder /app/venv /app/venv
ENV PATH="/app/venv/bin:$PATH"

# Copy the binary and key file from builder
COPY --from=builder /app/server .
COPY --from=builder /app/server-key.json .

# Create scripts directory and copy only required scripts
RUN mkdir -p scripts
COPY scripts/embed.py scripts/
COPY scripts/search.py scripts/

# Expose port
EXPOSE 8080

# Run the server
CMD ["./server"]