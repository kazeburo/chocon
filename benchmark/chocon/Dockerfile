FROM ubuntu:18.04
WORKDIR /root/go/src/github.com/kazeburo/chocon
RUN apt-get update && \
    apt-get install software-properties-common -y && \
    add-apt-repository ppa:longsleep/golang-backports && \
    apt-get update && \
    apt-get install golang-go build-essential git iproute2 apache2-utils curl -y
RUN mkdir -p /root/go/bin
RUN curl -s https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
ADD . .
RUN /root/go/bin/dep ensure
CMD ["go", "run", "chocon.go"]
