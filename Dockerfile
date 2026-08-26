# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS build
WORKDIR /src

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/migrate ./cmd/migrate

FROM alpine:3.20 AS base
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S medqueue && adduser -S medqueue -G medqueue
WORKDIR /app
COPY --from=build /out/ /app/bin/
COPY --from=build /src/migrations /app/migrations
COPY --from=build /src/docs /app/docs
USER medqueue

FROM base AS api
EXPOSE 8080
ENTRYPOINT ["/app/bin/api"]

FROM base AS worker
ENTRYPOINT ["/app/bin/worker"]

FROM base AS migrate
ENTRYPOINT ["/app/bin/migrate"]
