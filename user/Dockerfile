FROM golang:1.23.2 AS builder

WORKDIR /app
COPY ./shared /app/shared
COPY ./user /app/user

WORKDIR /app/user

RUN go mod tidy
RUN go build -v -o /usr/local/bin/user .

EXPOSE 8080

CMD ["user"]