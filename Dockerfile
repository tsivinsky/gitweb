FROM golang:1.21

WORKDIR /app

COPY . .
RUN go build -v -o /usr/local/bin/app main.go

CMD ["app"]
