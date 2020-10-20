FROM golang:1.14

WORKDIR /go/src/osu-api-proxy
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

VOLUME /cache

CMD ["osu-api-proxy"]
