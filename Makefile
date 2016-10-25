VERSION=0.4.0
LDFLAGS=-ldflags "-X main.Version=${VERSION}"
TARGETS_NOVENDOR=$(shell glide novendor)

all: chocon

.PHONY: chocon

glide:
	go get -u github.com/Masterminds/glide

bundle:
	glide install

chocon: chocon.go
	GO15VENDOREXPERIMENT=1 go build $(LDFLAGS) chocon.go

linux: chocon.go
	GOOS=linux GOARCH=amd64 GO15VENDOREXPERIMENT=1 go build $(LDFLAGS) chocon.go

fmt:
	@echo $(TARGETS_NOVENDOR) | xargs go fmt

dist:
	git archive --format tgz HEAD -o chocon-$(VERSION).tar.gz --prefix chocon-$(VERSION)/

clean:
	rm -rf chocon chocon-*.tar.gz

tag:
	git tag v${VERSION}
	git push origin v${VERSION}
	git push origin master


