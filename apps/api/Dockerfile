# syntax=docker/dockerfile:1

FROM golang:1.22 AS builder
WORKDIR /src
COPY . .
WORKDIR /src/apps/api
RUN go build -o /bin/api ./cmd/server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /bin/api ./api
EXPOSE 8080
ENV APP_SERVER_ADDR=0.0.0.0:8080
ENTRYPOINT ["/app/api"]
