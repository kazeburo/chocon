# chocon

**chocon** is a simple proxy server for persisting connections between upstream servers.

# Requirements

**chocon** requires Go1.7.3 or later.

# Installation

```
go get -u github.com/kazeburo/chocon
```

# Build

```
make bundle
make
```

# Run

```
chocon
```

# Usage

```
$ chocon -h
Usage:
chocon [OPTIONS]

Application Options:
-l, --listen=             address to bind (default: 0.0.0.0)
-p, --port=               Port number to bind (default: 3000)
--access-log-dir=     directory to store logfiles
--access-log-rotate=  Number of day before remove logs (default: 30)
-v, --version             Show version
-c, --keepalive-conns=    maximum keepalive connections for upstream (default: 2)
--read-timeout=       timeout of reading request (default: 30)
--write-timeout=      timeout of writing response (default: 90)
--proxy-read-timeout= timeout of reading response from upstream (default: 60)

Help Options:
-h, --help                Show this help message

```
