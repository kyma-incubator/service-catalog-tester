FROM golang:1.13.12-alpine3.12 as builder

ENV BASE_APP_DIR /go/src/github.com/kyma-incubator/service-catalog-tester/
WORKDIR ${BASE_APP_DIR}

COPY . ${BASE_APP_DIR}/

RUN go build -v -o health-proxy main.go
RUN mkdir /app && mv ./health-proxy /app/health-proxy

FROM alpine:3.12.0
WORKDIR /app

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

COPY --from=builder /app /app

CMD ["/app/health-proxy"]
