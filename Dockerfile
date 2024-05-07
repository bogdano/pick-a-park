ARG GO_VERSION=1
FROM golang:${GO_VERSION}-bookworm as builder
RUN apt-get update && apt-get install -y ca-certificates && update-ca-certificates

WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN go build -v -o /run-app .

FROM debian:bookworm

COPY --from=builder /run-app /usr/local/bin/
# the following lines caused a lot of pain to figure out
COPY ./pb_public /pb/pb_public
CMD ["run-app", "serve", "--http=0.0.0.0:8080"]
