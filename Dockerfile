FROM golang:1.22.5 AS builder
WORKDIR /app

RUN apt update && apt install -y make git

COPY go.mod go.sum ./
RUN go mod download -x

COPY . .

RUN make build
