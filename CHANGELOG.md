# Changelog

## [v0.12.6](https://github.com/kazeburo/chocon/compare/v0.12.5...v0.12.6) - 2025-09-13
- use tagpr for release by @kazeburo in https://github.com/kazeburo/chocon/pull/50
- update deps by @kazeburo in https://github.com/kazeburo/chocon/pull/52

## [v0.12.5](https://github.com/kazeburo/chocon/compare/v0.12.4...v0.12.5) - 2021-09-27
- update go by @kazeburo in https://github.com/kazeburo/chocon/pull/48

## [v0.12.4](https://github.com/kazeburo/chocon/compare/v0.12.3...v0.12.4) - 2021-08-05
- Go Modules: update dependencies. by @cubicdaiya in https://github.com/kazeburo/chocon/pull/47

## [v0.12.3](https://github.com/kazeburo/chocon/compare/v0.12.2...v0.12.3) - 2020-12-18
- support armv6 by @kazeburo in https://github.com/kazeburo/chocon/pull/46

## [v0.12.2](https://github.com/kazeburo/chocon/compare/v0.12.1...v0.12.2) - 2020-11-27
- doc: updated how to build. by @cubicdaiya in https://github.com/kazeburo/chocon/pull/44
- goreleaser by github actions by @kazeburo in https://github.com/kazeburo/chocon/pull/45

## [v0.12.1](https://github.com/kazeburo/chocon/compare/v0.12.0...v0.12.1) - 2020-04-20
- use Go Modules and update modules by @catatsuy in https://github.com/kazeburo/chocon/pull/40
- add GitHub Actions for CI by @catatsuy in https://github.com/kazeburo/chocon/pull/41
- add rewrite test by @kazeburo in https://github.com/kazeburo/chocon/pull/42
- Fixed log maxage by @kazeburo in https://github.com/kazeburo/chocon/pull/43

## [v0.12.0](https://github.com/kazeburo/chocon/compare/v0.11.0...v0.12.0) - 2020-04-08
- only change zap context when error by @kazeburo in https://github.com/kazeburo/chocon/pull/39

## [v0.11.0](https://github.com/kazeburo/chocon/compare/v0.10.6...v0.11.0) - 2020-02-19
- support --access-log-rotate-time by @kazeburo in https://github.com/kazeburo/chocon/pull/38

## [v0.10.6](https://github.com/kazeburo/chocon/compare/v0.10.5...v0.10.6) - 2019-06-11
- use xid. xid is good performance by @kazeburo in https://github.com/kazeburo/chocon/pull/36

## [v0.10.5](https://github.com/kazeburo/chocon/compare/v0.10.4...v0.10.5) - 2019-06-07
- optimized memory alloc by @kazeburo in https://github.com/kazeburo/chocon/pull/35

## [v0.10.4](https://github.com/kazeburo/chocon/compare/v0.10.1...v0.10.4) - 2019-05-14
- shutdown gracefully by @kazeburo in https://github.com/kazeburo/chocon/pull/34

## [v0.10.1](https://github.com/kazeburo/chocon/compare/v0.10.0...v0.10.1) - 2019-02-23
- Introduced pidfile by @kazeburo in https://github.com/kazeburo/chocon/pull/33

## [v0.10.0](https://github.com/kazeburo/chocon/compare/v0.9.4...v0.10.0) - 2019-02-22
- json access log by @kazeburo in https://github.com/kazeburo/chocon/pull/31
- improve proxyID and loop detect by @kazeburo in https://github.com/kazeburo/chocon/pull/32

## [v0.9.4](https://github.com/kazeburo/chocon/compare/v0.9.3...v0.9.4) - 2019-02-18

## [v0.9.3](https://github.com/kazeburo/chocon/compare/v0.9.2...v0.9.3) - 2019-02-18
- doc: update usage. by @cubicdaiya in https://github.com/kazeburo/chocon/pull/29
- upstream least_conn by @kazeburo in https://github.com/kazeburo/chocon/pull/30

## [v0.9.2](https://github.com/kazeburo/chocon/compare/v0.9.1...v0.9.2) - 2019-02-15
- enhance error log from proxy by @kazeburo in https://github.com/kazeburo/chocon/pull/28

## [v0.9.1](https://github.com/kazeburo/chocon/compare/v0.9.0...v0.9.1) - 2019-02-12
- upstream: imporved error hanling. by @cubicdaiya in https://github.com/kazeburo/chocon/pull/26
- Introduced max conns per host by @kazeburo in https://github.com/kazeburo/chocon/pull/27

## [v0.9.0](https://github.com/kazeburo/chocon/compare/v0.8.1...v0.9.0) - 2019-01-25
- introduced go-httpstats. by @cubicdaiya in https://github.com/kazeburo/chocon/pull/23
- refactor: printVersion(). by @cubicdaiya in https://github.com/kazeburo/chocon/pull/24
- update lestrrat libs by @kazeburo in https://github.com/kazeburo/chocon/pull/25

## [v0.8.1](https://github.com/kazeburo/chocon/compare/v0.8.0...v0.8.1) - 2019-01-24
- Use dep by @kazeburo in https://github.com/kazeburo/chocon/pull/22

## [v0.8.0](https://github.com/kazeburo/chocon/compare/v0.7.1...v0.8.0) - 2019-01-16
- bumped version to 0.7.1. by @cubicdaiya in https://github.com/kazeburo/chocon/pull/14
- Fix grammar split past tense by @nekop in https://github.com/kazeburo/chocon/pull/15
- Support custom port by @nekop in https://github.com/kazeburo/chocon/pull/16
- Support --upstream by @kazeburo in https://github.com/kazeburo/chocon/pull/17

## [v0.7.1](https://github.com/kazeburo/chocon/compare/v0.7.0...v0.7.1) - 2017-11-27
- net.SplitHostPort raised error when r.Host did not have `:port` by @kazeburo in https://github.com/kazeburo/chocon/pull/10
- typo fixed. by @cubicdaiya in https://github.com/kazeburo/chocon/pull/11
- change variable names and the package names for golint by @catatsuy in https://github.com/kazeburo/chocon/pull/12
- optimize: made ignoredHeaderNames map. by @cubicdaiya in https://github.com/kazeburo/chocon/pull/13

## [v0.7.0](https://github.com/kazeburo/chocon/compare/v0.6.0...v0.7.0) - 2017-05-24
- Fix build by @mattn in https://github.com/kazeburo/chocon/pull/8
- split host and port by @kazegusuri in https://github.com/kazeburo/chocon/pull/9
