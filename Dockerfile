# syntax=docker/dockerfile:1
FROM golang:1.26.6-alpine AS builder

ARG SERVICE=api
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/service ./cmd/${SERVICE}

FROM alpine:3.23.5
RUN addgroup -S app && adduser -S -G app app
COPY --from=builder /out/service /usr/local/bin/service
USER app
ENTRYPOINT ["/usr/local/bin/service"]
