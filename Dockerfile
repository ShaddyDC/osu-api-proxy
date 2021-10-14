FROM golang:1.17-alpine AS builder

ARG BuildCommit
ARG BuildTime

RUN echo ${BuildCommit}
RUN echo ${BuildTime}

WORKDIR /go/src/osu-api-proxy
COPY . .

RUN go get -d -v ./... && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-X 'main.BuildCommit=$BuildCommit' -X 'main.BuildTime=$BuildTime'" .

FROM alpine

WORKDIR /osu-api-proxy

COPY --from=builder /go/src/osu-api-proxy ./

# Default ports
EXPOSE 8126 8125 8127

CMD ["./osu-api-proxy"]
