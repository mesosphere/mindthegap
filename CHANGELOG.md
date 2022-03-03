# Changelog

### [0.6.4](https://github.com/mesosphere/mindthegap/compare/v0.6.3...v0.6.4) (2022-03-03)


### Bug Fixes

* Improved log output for manifest errors ([f3d1886](https://github.com/mesosphere/mindthegap/commit/f3d1886db7257f644c21790bd9002e79b9f685a7))

### [0.6.3](https://github.com/mesosphere/mindthegap/compare/v0.6.0...v0.6.3) (2022-03-03)


### Features

* add command descriptions ([#47](https://github.com/mesosphere/mindthegap/issues/47)) ([8d485a7](https://github.com/mesosphere/mindthegap/commit/8d485a75695ff794448cfc7a34ee41f02a8eeb09))
* Fall back to image from local Docker runtime ([9d003ae](https://github.com/mesosphere/mindthegap/commit/9d003ae8d94afcbd01de6462bcabe3d3401b9451))
* Gzip image bundle tarball ([#37](https://github.com/mesosphere/mindthegap/issues/37)) ([685372e](https://github.com/mesosphere/mindthegap/commit/685372e45f1edddc7084e48364a173de2168121c))
* Sort images for deterministic ordering of create and push ([9b89b26](https://github.com/mesosphere/mindthegap/commit/9b89b2608d07be8c1d8c9c2f3537c4008f4ed9b2))
* Support basic auth settins in docker config ([1d8d5c4](https://github.com/mesosphere/mindthegap/commit/1d8d5c402a3a66d4756706a0ee76fd567c743c03))
* Support registry creds for image push ([9de127c](https://github.com/mesosphere/mindthegap/commit/9de127c8c381feecaefb14e56699d8f8654e8f8e))
* Upgrade to skopeo v1.6.1 ([f06810e](https://github.com/mesosphere/mindthegap/commit/f06810e67975b51a4f100ecacb63348870925d11))


### Bug Fixes

* Always cleanup temporary directory ([#41](https://github.com/mesosphere/mindthegap/issues/41)) ([9ea5f97](https://github.com/mesosphere/mindthegap/commit/9ea5f97f015afc79ecca52a290f43a265d7c61e2))
* Ensure skopeo policy is unpacked before copy ([a060b30](https://github.com/mesosphere/mindthegap/commit/a060b30d56ecb142231928df16d054fde292e6e2))
* Fix registry log in error checking ([#38](https://github.com/mesosphere/mindthegap/issues/38)) ([3ebe46f](https://github.com/mesosphere/mindthegap/commit/3ebe46f20a511736ab3bba1c77c60263fbf590bc))
* Remove deadlock in cleanup ([09bde3c](https://github.com/mesosphere/mindthegap/commit/09bde3c37bef3f2d8396d4448714e4bed6ed85c7))
* Upgrade containerd dep to fix CVEs ([3e44a63](https://github.com/mesosphere/mindthegap/commit/3e44a63a7855411adfd5d0ef12b2c2dd96c8f46e))


### Build System

* release 0.6.1 ([ca0cbd2](https://github.com/mesosphere/mindthegap/commit/ca0cbd249f1b6dd7070e4752ecb31c632a0e2fa5))
* release 0.6.3 ([f4a605b](https://github.com/mesosphere/mindthegap/commit/f4a605b99d4ac350d47825d2e0424ee5d35e4bf9))

### [0.6.1](https://github.com/mesosphere/mindthegap/compare/v0.6.0...v0.6.1) (2022-03-03)


### Features

* add command descriptions ([#47](https://github.com/mesosphere/mindthegap/issues/47)) ([8d485a7](https://github.com/mesosphere/mindthegap/commit/8d485a75695ff794448cfc7a34ee41f02a8eeb09))
* Fall back to image from local Docker runtime ([9d003ae](https://github.com/mesosphere/mindthegap/commit/9d003ae8d94afcbd01de6462bcabe3d3401b9451))
* Gzip image bundle tarball ([#37](https://github.com/mesosphere/mindthegap/issues/37)) ([685372e](https://github.com/mesosphere/mindthegap/commit/685372e45f1edddc7084e48364a173de2168121c))
* Sort images for deterministic ordering of create and push ([9b89b26](https://github.com/mesosphere/mindthegap/commit/9b89b2608d07be8c1d8c9c2f3537c4008f4ed9b2))
* Support basic auth settins in docker config ([1d8d5c4](https://github.com/mesosphere/mindthegap/commit/1d8d5c402a3a66d4756706a0ee76fd567c743c03))
* Support registry creds for image push ([9de127c](https://github.com/mesosphere/mindthegap/commit/9de127c8c381feecaefb14e56699d8f8654e8f8e))
* Upgrade to skopeo v1.6.1 ([f06810e](https://github.com/mesosphere/mindthegap/commit/f06810e67975b51a4f100ecacb63348870925d11))


### Bug Fixes

* Always cleanup temporary directory ([#41](https://github.com/mesosphere/mindthegap/issues/41)) ([9ea5f97](https://github.com/mesosphere/mindthegap/commit/9ea5f97f015afc79ecca52a290f43a265d7c61e2))
* Ensure skopeo policy is unpacked before copy ([a060b30](https://github.com/mesosphere/mindthegap/commit/a060b30d56ecb142231928df16d054fde292e6e2))
* Fix registry log in error checking ([#38](https://github.com/mesosphere/mindthegap/issues/38)) ([3ebe46f](https://github.com/mesosphere/mindthegap/commit/3ebe46f20a511736ab3bba1c77c60263fbf590bc))
* Remove deadlock in cleanup ([09bde3c](https://github.com/mesosphere/mindthegap/commit/09bde3c37bef3f2d8396d4448714e4bed6ed85c7))
* Upgrade containerd dep to fix CVEs ([3e44a63](https://github.com/mesosphere/mindthegap/commit/3e44a63a7855411adfd5d0ef12b2c2dd96c8f46e))


### Build System

* release 0.6.1 ([ca0cbd2](https://github.com/mesosphere/mindthegap/commit/ca0cbd249f1b6dd7070e4752ecb31c632a0e2fa5))
