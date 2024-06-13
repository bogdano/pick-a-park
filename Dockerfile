ARG GO_VERSION=1
FROM golang:${GO_VERSION}-bookworm as builder
RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN ls -la /etc/ssl/certs


WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o /run-app .

FROM debian:bookworm

COPY --from=builder /run-app /usr/local/bin/
COPY --from=builder /etc/ssl/certs /etc/ssl/certs

COPY ./pb_public /pb_public
CMD ["run-app", "serve", "--http=0.0.0.0:8080"]
