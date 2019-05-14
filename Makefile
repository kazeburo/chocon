VERSION=0.10.2
LDFLAGS=-ldflags "-X main.Version=${VERSION}"

all: chocon

.PHONY: chocon

bundle:
	dep ensure

update:
	dep ensure -update

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

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin master
	goreleaser --rm-dist
