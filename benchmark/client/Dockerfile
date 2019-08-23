FROM ubuntu:18.04
WORKDIR /work
RUN apt-get update && \
    apt-get install software-properties-common -y && \
    add-apt-repository ppa:longsleep/golang-backports && \
    apt-get update && \
    apt-get install golang-go build-essential git iproute2 apache2-utils -y
RUN go get -u github.com/rakyll/hey
