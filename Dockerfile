FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY src/ ./
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /webhook-hub .

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -g '' appuser

WORKDIR /app

COPY --from=builder /webhook-hub /app/webhook-hub
COPY entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

ENV PORT=80

EXPOSE 80

ENTRYPOINT ["/app/entrypoint.sh"]
