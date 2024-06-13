# Build container
FROM golang:1.21 AS build

ARG VERSION

COPY . /home/src
WORKDIR /home/src

RUN CGO_ENABLED=0 go build -v -o . -ldflags="-w -s" ./...

# Runtime container
FROM alpine as ethereum-publisher

RUN apk add --update --no-cache \
    ca-certificates \
    bash

WORKDIR /home/app
COPY --from=build /home/src/ethereum-publisher .
RUN chmod +x /home/app/ethereum-publisher

CMD ["./ethereum-publisher", "start"]
