# --- Build stage ---
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o backer-backend .

# --- Run stage ---
FROM alpine:3.20
RUN apk --no-cache add ca-certificates && \
    adduser -D -u 1000 appuser
WORKDIR /app
COPY --from=builder /app/backer-backend .
COPY --from=builder /app/web/templates ./web/templates
COPY --from=builder /app/web/assets ./web/assets
RUN chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
CMD ["./backer-backend"]