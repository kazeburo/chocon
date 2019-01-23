VERSION=0.8.0
LDFLAGS=-ldflags "-X main.Version=${VERSION}"
TARGETS_NOVENDOR=$(shell glide novendor)

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
	go test -v $(TARGETS_NOVENDOR)

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
