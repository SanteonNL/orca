# syntax=docker/dockerfile:1

FROM golang:1.25.7-alpine AS build

LABEL maintainer="info@reinkrul.nl"

ENV GO111MODULE on
ENV GOPATH /

COPY go.mod .
COPY go.sum .
RUN go mod download && go mod verify
COPY . .

RUN mkdir /app
RUN go build -o /app/bin .

# ## Deploy
FROM gcr.io/distroless/base-debian11:debug
# Connect to shell via /busybox/sh

COPY --from=build /app/bin /app/bin

WORKDIR /app

# Run the app binary when we run the container
ENTRYPOINT ["/app/bin"]
