FROM alpine:3.20

# Forked from https://github.com/pozgo/docker-stress-ng
LABEL \
    author="David Aronchick" \
    email="help@expanso.io" \
    version="0.0.3" \
    description="stress-ng in a docker for alpine"

ENV SHELL=/bin/bash

RUN \
    apk add --no-cache stress-ng

ENTRYPOINT ["/usr/bin/stress-ng"]
