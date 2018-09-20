FROM golang:1.10.2-alpine3.7 as builder

ENV BASE_APP_DIR /go/src/github.com/kyma-incubator/service-catalog-tester
WORKDIR ${BASE_APP_DIR}

#
# Copy files
#

COPY . ${BASE_APP_DIR}/

#
# Build app
#

RUN go build -v -o main .
RUN mkdir /app && mv ./main /app/main

FROM alpine:3.8
LABEL source = git@github.com:kyma-incubator/service-catalog-tester.git
WORKDIR /app

#
# Install certificates
#

RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

#
# Copy binary
#

COPY --from=builder /app /app

#
# Run app
#

CMD ["/app/main"]
