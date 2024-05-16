FROM golang:1.22.2-alpine as builder

LABEL maintainer="info@reinkrul.nl"

ENV GO111MODULE on
ENV GOPATH /

COPY go.mod .
COPY go.sum .
RUN go mod download && go mod verify
COPY . .

RUN mkdir /app
RUN go build -o /app/bin .

# alpine
FROM alpine:3.19.1
RUN apk update \
  && apk add --no-cache \
             tzdata \
             curl \
  && update-ca-certificates
COPY --from=builder /app/bin /app/bin

WORKDIR /app

ENTRYPOINT ["/app/bin"]
