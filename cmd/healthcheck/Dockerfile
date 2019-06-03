FROM golang:1.10.4-alpine3.8 as builder

ENV BASE_APP_DIR /go/src/github.com/kyma-incubator/service-catalog-tester/
WORKDIR ${BASE_APP_DIR}

COPY . ${BASE_APP_DIR}/

RUN go build -v -o main main.go
RUN mkdir /app && mv ./main /app/main

FROM alpine:3.8
WORKDIR /app

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

COPY --from=builder /app /app

CMD ["/app/main"]
