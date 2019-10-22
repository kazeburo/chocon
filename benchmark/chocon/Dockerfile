FROM ubuntu:18.04
WORKDIR /root/go/src/github.com/kazeburo/chocon
RUN apt-get update && \
    apt-get install software-properties-common -y && \
    add-apt-repository ppa:longsleep/golang-backports && \
    apt-get update && \
    apt-get install golang-go build-essential git curl -y
RUN mkdir -p /root/go/bin
RUN curl -s https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
ADD Gopkg.toml Gopkg.lock ./
RUN /root/go/bin/dep ensure --vendor-only
ADD . .
CMD ["go", "run", "chocon.go", "--insecure", "--port=80", "--max-conns-per-host=5000"]
