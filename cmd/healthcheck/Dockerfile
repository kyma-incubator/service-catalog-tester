FROM golang:1.13.12-alpine3.12 as builder

ENV BASE_APP_DIR /go/src/github.com/kyma-incubator/service-catalog-tester/
WORKDIR ${BASE_APP_DIR}

COPY . ${BASE_APP_DIR}/

RUN CGO_ENABLED=0 go build -v -o health-proxy main.go
RUN mkdir /app && mv ./health-proxy /app/health-proxy

# Get latest CA certs
FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM scratch
WORKDIR /app

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /app /app

CMD ["/app/health-proxy"]
