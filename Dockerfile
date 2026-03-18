# Build Stage
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o velarix main.go

# Run Stage
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/velarix .
# Create data directory for BadgerDB persistence
RUN mkdir -p /root/velarix.data
EXPOSE 8080
ENV VELARIX_API_KEY=""
CMD ["./velarix"]