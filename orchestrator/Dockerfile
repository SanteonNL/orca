FROM golang:1.22.2-alpine

LABEL maintainer="info@reinkrul.nl"

ENV GO111MODULE on
ENV GOPATH /

COPY go.mod .
COPY go.sum .
RUN go mod download && go mod verify
COPY . .

RUN go build -o /app .

ENTRYPOINT ["/app"]
