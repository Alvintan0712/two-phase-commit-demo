FROM golang:1.23.2 AS builder

WORKDIR /app
COPY . .

RUN go mod tidy
RUN go build -v -o /usr/local/bin/gateway .

EXPOSE 8000

CMD ["gateway"]