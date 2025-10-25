# syntax=docker/dockerfile:1

FROM golang:1.22 AS builder
WORKDIR /src
COPY . .
WORKDIR /src/apps/worker
RUN go build -o /bin/worker ./cmd/worker

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /bin/worker ./worker
ENTRYPOINT ["/app/worker"]
