FROM golang:1.19-alpine

RUN apk update
RUN apk add alpine-sdk

WORKDIR /gormigrate
COPY . .

RUN go mod download
