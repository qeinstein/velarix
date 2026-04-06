# Velarix Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o velarix .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/velarix .
EXPOSE 8080

CMD ["./velarix", "--lite"]
