# Build Stage
FROM golang:1.22-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o lazycommit main.go

# Runtime Stage
FROM ubuntu:22.04

# Prevent interactive prompts during apt-get
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install -y \
    git \
    openssh-client \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/lazycommit .
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Initialize config
RUN mkdir -p /root/.lazycommit/logs
RUN echo '[global]\n\
push_delay_seconds = 5\n\
\n\
[[repos]]\n\
path = "/root/test-repo"\n\
enabled = true\n\
lazy_push = true' > /root/.lazycommit/config.toml

# Use entrypoint to setup test repo
ENTRYPOINT ["/entrypoint.sh"]

# Run the daemon in the foreground
CMD ["./lazycommit", "daemon"]
