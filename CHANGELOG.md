# Changelog

## [0.17.2](https://github.com/mesosphere/mindthegap/compare/v0.17.1...v0.17.2) (2022-07-27)


### Bug Fixes

* Use ocischema.Manifest for plain image details ([#145](https://github.com/mesosphere/mindthegap/issues/145)) ([976a253](https://github.com/mesosphere/mindthegap/commit/976a2539d0c9241689077faa1abebdef629ae161))

## [0.17.1](https://github.com/mesosphere/mindthegap/compare/v0.17.0...v0.17.1) (2022-07-01)


### Build System

* release v0.17.1 ([49eb8a2](https://github.com/mesosphere/mindthegap/commit/49eb8a245c5b25c15a9ac0a06e5617761c690be8))

## [0.17.0](https://github.com/mesosphere/mindthegap/compare/v0.16.0...v0.17.0) (2022-06-14)


### Features

* Build skopeo with go v1.18 ([9d04798](https://github.com/mesosphere/mindthegap/commit/9d0479845fa34337ab56ae4a604b0147a6e061bb))
* go v1.18 ([#110](https://github.com/mesosphere/mindthegap/issues/110)) ([f84ffa3](https://github.com/mesosphere/mindthegap/commit/f84ffa3019590f441fdfc935f112e3c7f049b6fc))


### Bug Fixes

* Disable upx for all platforms ([#129](https://github.com/mesosphere/mindthegap/issues/129)) ([f5496c3](https://github.com/mesosphere/mindthegap/commit/f5496c373f195e6e639d4d5911b68ad37c81ccc6))
* Upgrade containerd dep to v1.6.6 to fix CVE ([#128](https://github.com/mesosphere/mindthegap/issues/128)) ([638f80e](https://github.com/mesosphere/mindthegap/commit/638f80ef5d991605fb4c0a369fee2f6aef46cd52))

## [0.16.0](https://github.com/mesosphere/mindthegap/compare/v0.15.2...v0.16.0) (2022-05-09)


### Features

* Skopeo v1.8.0 ([c8bdd81](https://github.com/mesosphere/mindthegap/commit/c8bdd8135908de865eb8c07fdb7bd3320978ef0b))

### [0.15.2](https://github.com/mesosphere/mindthegap/compare/v0.15.1...v0.15.2) (2022-04-25)


### Bug Fixes

* prevent 404 on ECR during image creation ([#100](https://github.com/mesosphere/mindthegap/issues/100)) ([e000ef7](https://github.com/mesosphere/mindthegap/commit/e000ef7c83441c5e10a83eccfa5f4888fa5e310f))

### [0.15.1](https://github.com/mesosphere/mindthegap/compare/v0.15.0...v0.15.1) (2022-04-05)


### Bug Fixes

* commitDate in goreleaser ([#92](https://github.com/mesosphere/mindthegap/issues/92)) ([017d2f3](https://github.com/mesosphere/mindthegap/commit/017d2f36da2eb61b556f034e3e3232bd84ce7796))

## [0.15.0](https://github.com/mesosphere/mindthegap/compare/v0.14.0...v0.15.0) (2022-03-31)


### Features

* skopeo v1.7.0 ([9440c2b](https://github.com/mesosphere/mindthegap/commit/9440c2b82ab10c6526fe34abcaec83deac0150f9))


### Bug Fixes

* Fix import image-bundle command ([#88](https://github.com/mesosphere/mindthegap/issues/88)) ([73e7f6c](https://github.com/mesosphere/mindthegap/commit/73e7f6c2e7d48fc12c6325607b0486cda810ad48))

## [0.14.0](https://github.com/mesosphere/mindthegap/compare/v0.13.1...v0.14.0) (2022-03-22)


### Features

* Auto-create ECR registries if they do not exist ([#79](https://github.com/mesosphere/mindthegap/issues/79)) ([b595607](https://github.com/mesosphere/mindthegap/commit/b595607e523497480998a4d35522c8d3553635ba))

### [0.13.1](https://github.com/mesosphere/mindthegap/compare/v0.13.0...v0.13.1) (2022-03-18)


### Bug Fixes

* Do not upx pack darwin binaries ([503d726](https://github.com/mesosphere/mindthegap/commit/503d72631ef1220244c578c518bdb3110026eaa6))

## [0.13.0](https://github.com/mesosphere/mindthegap/compare/v0.12.0...v0.13.0) (2022-03-18)


### Build System

* release 0.13.0 ([f7bed9a](https://github.com/mesosphere/mindthegap/commit/f7bed9a41135e142ee146ba433fa7f0fdc2bad62))

## [0.12.0](https://github.com/mesosphere/mindthegap/compare/v0.11.0...v0.12.0) (2022-03-15)


### Features

* Add username and password auth to helm repository config ([#65](https://github.com/mesosphere/mindthegap/issues/65)) ([8f74681](https://github.com/mesosphere/mindthegap/commit/8f74681596f945c0571f8e8ddb8f6dd7a6ce6912))

## [0.11.0](https://github.com/mesosphere/mindthegap/compare/v0.10.0...v0.11.0) (2022-03-03)


### Features

* Sort images for deterministic ordering of create and push ([adb29f8](https://github.com/mesosphere/mindthegap/commit/adb29f8f96ccd3606c60acc2051239ba9890f09b))


### Bug Fixes

* Upgrade containerd dep to fix CVEs ([0079ee8](https://github.com/mesosphere/mindthegap/commit/0079ee8704af91d30109974babf9f61244daaaf0))

## [0.10.0](https://github.com/mesosphere/mindthegap/compare/v0.9.1...v0.10.0) (2022-03-03)


### Features

* Upgrade to skopeo v1.6.1 ([009def6](https://github.com/mesosphere/mindthegap/commit/009def69cdb2e0e1fe4721fa183d5c0be911c467))
