FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY src/ ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /webhook-hub .

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /webhook-hub /app/webhook-hub

ENV PORT=8080

EXPOSE 8080

CMD ["/app/webhook-hub"]
