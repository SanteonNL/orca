# syntax=docker/dockerfile:1

FROM golang:1.22.2-alpine AS build

LABEL maintainer="info@reinkrul.nl"

ENV GO111MODULE on
ENV GOPATH /

COPY go.mod .
COPY go.sum .
RUN go mod download && go mod verify
COPY . .

RUN mkdir /app
RUN go build -o /app/bin .


# Run the app binary when we run the container

# ## Deploy
FROM gcr.io/distroless/base-debian11:debug  
# Connect to shell via /busybox/sh

WORKDIR /

COPY --from=build /app /app

WORKDIR /app

ENTRYPOINT ["/app/bin"]
