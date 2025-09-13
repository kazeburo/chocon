VERSION=0.12.6
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION}"

all: chocon

chocon: chocon.go
	go build $(LDFLAGS) chocon.go

linux: chocon.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) chocon.go

check:
	go test ./...

fmt:
	go fmt ./...

clean:
	rm -rf chocon chocon-*.tar.gz

