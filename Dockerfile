# Build stage
FROM golang:1.24.1-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o server .

# Run stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/server .
EXPOSE 8080
CMD ["./server"]