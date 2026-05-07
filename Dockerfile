# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api/

# Runtime stage
FROM alpine:3.19

COPY --from=builder /api /api

EXPOSE 8080

CMD ["/api"]
