FROM alpine:latest

RUN apk update && apk add --no-cache unbound drill

COPY unbound.conf /etc/unbound/

USER nobody

ENTRYPOINT ["unbound"]

HEALTHCHECK --interval=30s --timeout=30s --start-period=5s --retries=3 \
    CMD [ "drill", "-p", "5053", "nlnetlabs.nl", "@127.0.0.1" ]
