# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /gp-takeout-resolver .

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache perl exiftool

COPY --from=builder /gp-takeout-resolver /usr/local/bin/gp-takeout-resolver

ENTRYPOINT ["gp-takeout-resolver"]
