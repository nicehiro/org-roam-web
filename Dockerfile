FROM golang:1.21-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /org-roam-web .

FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY --from=builder /org-roam-web /usr/local/bin/org-roam-web

ENTRYPOINT ["/usr/local/bin/org-roam-web"]
