# syntax=docker/dockerfile:1

FROM golang:1.25.5-alpine AS build

ENV GO111MODULE=on
ENV GOPATH=/

COPY go.mod .
COPY go.sum .
RUN go mod download && go mod verify
COPY . .

RUN mkdir /app
RUN go build -o /app/bin .

# ## Deploy
FROM gcr.io/distroless/base-debian13:debug
# Connect to shell via /busybox/sh

COPY --from=build /app/bin /app/bin

WORKDIR /app

# Use a non-root user to run the application
USER 1000
# Run the app binary when we run the container
ENTRYPOINT ["/app/bin"]
