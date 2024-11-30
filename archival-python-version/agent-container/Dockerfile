FROM alpine:latest

RUN apk add --no-cache bash

COPY bacalhau_v1.3.2_linux_amd64.tar.gz /tmp
RUN tar -xvf /tmp/bacalhau_v1.3.2_linux_amd64.tar.gz -C /tmp
RUN mv /tmp/bacalhau /usr/local/bin/bacalhau

ARG BACALHAU_NODE_NETWORK_ORCHESTRATORS
ENV BACALHAU_NODE_NETWORK_ORCHESTRATORS=${BACALHAU_NODE_NETWORK_ORCHESTRATORS}

COPY run.sh /run.sh
RUN chmod +x /run.sh

CMD ["/run.sh"]