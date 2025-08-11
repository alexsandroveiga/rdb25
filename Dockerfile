
FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o rdb2025 .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/rdb2025 .

EXPOSE 9999

ENTRYPOINT ["./rdb2025"]
