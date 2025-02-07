# syntax=docker/dockerfile:1

FROM scratch AS java-base
ADD https://github.com/hapifhir/org.hl7.fhir.core/releases/download/6.5.7/validator_cli.jar /fhir_validator_cli.jar

FROM golang:1.23.6-alpine AS build

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
FROM gcr.io/distroless/java21-debian12
# Connect to shell via /busybox/sh

COPY --from=build /app/bin /app/bin
COPY --from=java-base /fhir_validator_cli.jar /fhir_validator_cli.jar

WORKDIR /app

# Run the app binary when we run the container
ENTRYPOINT ["/app/bin"]
