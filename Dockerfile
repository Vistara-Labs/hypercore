FROM golang:1.23.1 AS builder
WORKDIR /app

RUN apt update && apt install -y make git

RUN CGO_ENABLED=0 go install github.com/awslabs/tc-redirect-tap/cmd/tc-redirect-tap@latest

COPY go.mod go.sum ./
RUN go mod download -x

COPY . .

RUN make build
