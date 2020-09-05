FROM golang:1.15.1

WORKDIR /gormigrate
COPY . .

RUN go mod download
