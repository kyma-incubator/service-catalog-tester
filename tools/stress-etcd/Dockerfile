FROM alpine:3.8

RUN apk add --no-cache \
    curl \
    bash \
    coreutils

ADD stress.sh /bin/

CMD ["/bin/stress.sh"]
