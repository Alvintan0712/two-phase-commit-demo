FROM golang:1.23.2 AS builder

WORKDIR /app
COPY ./shared /app/shared
COPY ./order /app/order

WORKDIR /app/order

RUN go mod tidy
RUN go build -v -o /usr/local/bin/order .

EXPOSE 8081

CMD ["order"]