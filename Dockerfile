FROM golang:1.12 as build

ENV GO111MODULE=on

RUN mkdir /go/src/cleanupDisks
WORKDIR /go/src/cleanupDisks
RUN go mod init
COPY go.mod .
COPY go.sum .


# Get dependancies - will also be cached if we won't change mod/sum
RUN go mod download

# COPY the source code as the last step
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o /usr/local/bin/cleanupDisks


FROM debian:stretch-slim
RUN apt-get -qqq update \
    && apt-get -qqq -y install ca-certificates \
    && update-ca-certificates
COPY --from=build /usr/local/bin/cleanupDisks /usr/local/bin/cleanupDisks

ENTRYPOINT ["cleanupDisks"]
