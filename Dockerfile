FROM golang:1.21

WORKDIR /app

RUN apt-get update && \
  apt-get upgrade -y && \
  apt-get install -y git

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/app main.go

CMD ["app"]
