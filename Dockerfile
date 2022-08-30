FROM golang:1.18 as build-root

WORKDIR /go/src/github.com/cyverse-de/monitoring-api
COPY . .

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN go build --buildvcs=false .
RUN go clean -cache -modcache
RUN cp ./monitoring-api /bin/monitoring-api

ENTRYPOINT ["monitoring-api"]

EXPOSE 60000
