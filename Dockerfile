FROM golang:1.12-alpine as builder

ENV BASE_APP_DIR /go/src/github.com/kyma-incubator/service-catalog-tester
WORKDIR ${BASE_APP_DIR}

COPY . ${BASE_APP_DIR}/

RUN apk update && apk add ca-certificates

RUN go build -v -o main .
RUN mkdir /app && mv ./main /app/main

FROM scratch
LABEL source = git@github.com:kyma-incubator/service-catalog-tester.git
WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app /app

CMD ["/app/main"]
