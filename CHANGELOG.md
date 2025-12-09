# Changelog

## [0.1.14](https://github.com/cccteam/logger/compare/v0.1.13...v0.1.14) (2025-12-09)


### Code Upgrade

* go =&gt; 1.25.5 & dependencies ([#86](https://github.com/cccteam/logger/issues/86)) ([07210a3](https://github.com/cccteam/logger/commit/07210a36af4e8577059f5c39c588f5bea762d142))

## [0.1.13](https://github.com/cccteam/logger/compare/v0.1.12...v0.1.13) (2025-11-26)


### Code Refactoring

* Rename Ctx() -&gt; FromCtx() and Req() -&gt; FromReq() ([#80](https://github.com/cccteam/logger/issues/80)) ([352dc92](https://github.com/cccteam/logger/commit/352dc92baa29387b7567315e48259d12b934c970))

## [0.1.12](https://github.com/cccteam/logger/compare/v0.1.11...v0.1.12) (2024-07-11)


### Bug Fixes

* Fix gRPC metadata security issue ([#73](https://github.com/cccteam/logger/issues/73)) ([03fdeae](https://github.com/cccteam/logger/commit/03fdeaea3c40c1973bb84b74abe38fd5eef23e0c))

## [0.1.11](https://github.com/cccteam/logger/compare/v0.1.10...v0.1.11) (2024-07-03)


### Code Upgrade

* Update Go version to 1.22.5 to address GO-2024-2963 ([#68](https://github.com/cccteam/logger/issues/68)) ([8ca1e4f](https://github.com/cccteam/logger/commit/8ca1e4ff5ea69999d580c4a94d3a4ccedf6a3b95))

## [0.1.10](https://github.com/cccteam/logger/compare/v0.1.9...v0.1.10) (2024-06-05)


### Code Upgrade

* Go version 1.22.4 for vulnerability GO-2024-2887 ([#60](https://github.com/cccteam/logger/issues/60)) ([fb44d5b](https://github.com/cccteam/logger/commit/fb44d5b96845e6b2698aeae886ec1481c7e9a49c))

## [0.1.9](https://github.com/cccteam/logger/compare/v0.1.8...v0.1.9) (2024-05-10)


### Features

* Add semantic pull request titles workflow ([#53](https://github.com/cccteam/logger/issues/53)) ([8ed4503](https://github.com/cccteam/logger/commit/8ed45030d7788556fb7d337b6b3b7468c41fdbf3))


### Bug Fixes

* Add missing Release Please changelog section type ([#55](https://github.com/cccteam/logger/issues/55)) ([df92379](https://github.com/cccteam/logger/commit/df92379050960e4fd3ccfffc64ba7b91b1420a0e))


### Code Upgrade

* Go version 1.22.3 and dependencies ([#52](https://github.com/cccteam/logger/issues/52)) ([5df6d1a](https://github.com/cccteam/logger/commit/5df6d1aadd44e783208c504c1bfda447f0c703fc))
* Upgrade golang-ci workflow to 3.0.0 ([#53](https://github.com/cccteam/logger/issues/53)) ([8ed4503](https://github.com/cccteam/logger/commit/8ed45030d7788556fb7d337b6b3b7468c41fdbf3))

## [0.1.8](https://github.com/cccteam/logger/compare/v0.1.7...v0.1.8) (2024-04-15)


### Features

* Add support for Flusher interface ([#48](https://github.com/cccteam/logger/issues/48)) ([4739e55](https://github.com/cccteam/logger/commit/4739e555078f538fc50b71b8de90093bbaaddaee))


### Dependencies

* upgrade Go to version 1.22.2 with deps ([#45](https://github.com/cccteam/logger/issues/45)) ([2cc5beb](https://github.com/cccteam/logger/commit/2cc5beb85c131c547b4b8670bf77d62d1f2f0d7c))

## [0.1.7](https://github.com/cccteam/logger/compare/v0.1.6...v0.1.7) (2024-03-06)


### Dependencies

* upgrade to Go version 1.21.8 ([8e13d7b](https://github.com/cccteam/logger/commit/8e13d7b8dcc8a3a74f34dac3e55fb07de467bb5a))

## [0.1.6](https://github.com/cccteam/logger/compare/v0.1.5...v0.1.6) (2024-02-20)


### Features

* Expose TraceID of the logger ([#34](https://github.com/cccteam/logger/issues/34)) ([ea6ad1e](https://github.com/cccteam/logger/commit/ea6ad1e18c17ba207e6d4b446c0c8b6337f62ad3))


### Bug Fixes

* Change Severity Classification for 400 Status Codes from Error to Info ([#34](https://github.com/cccteam/logger/issues/34)) ([ea6ad1e](https://github.com/cccteam/logger/commit/ea6ad1e18c17ba207e6d4b446c0c8b6337f62ad3))

## [0.1.5](https://github.com/cccteam/logger/compare/v0.1.4...v0.1.5) (2024-01-19)


### Features

* Add public function for adding a logger to a context ([#25](https://github.com/cccteam/logger/issues/25)) ([43f584e](https://github.com/cccteam/logger/commit/43f584e9b3b2e78a57abb274ff50521b23862386))

## [0.1.4](https://github.com/cccteam/logger/compare/v0.1.3...v0.1.4) (2023-12-07)


### Features

* Support adding attributes to logs dynamically ([#19](https://github.com/cccteam/logger/issues/19)) ([4b8c36b](https://github.com/cccteam/logger/commit/4b8c36bfe00f853e3b4a201378a06fbe6faf708e))

## [0.1.3](https://github.com/cccteam/logger/compare/v0.1.2...v0.1.3) (2023-11-06)


### Features

* Add request logging to ConsoleExporter ([#11](https://github.com/cccteam/logger/issues/11)) ([c5d641d](https://github.com/cccteam/logger/commit/c5d641d585f29bc3d7a115621ffb5c04160e02c9))

## [0.1.2](https://github.com/cccteam/logger/compare/v0.1.1...v0.1.2) (2023-10-05)


### Features

* AWS Log Exporter ([d3a0f80](https://github.com/cccteam/logger/commit/d3a0f80ca304d722a7689a47a12d6cca24f0dbd0))

## [0.1.1](https://github.com/jtwatson/logger/compare/v0.1.0...v0.1.1) (2023-08-10)


### Features

* Add HTTP Method to console logs ([#28](https://github.com/jtwatson/logger/issues/28)) ([1f6a5a0](https://github.com/jtwatson/logger/commit/1f6a5a0695af817137225720fe5c5f5086852b76))

## [0.1.0](https://github.com/jtwatson/logger/compare/v0.0.3...v0.1.0) (2023-01-25)


### ⚠ BREAKING CHANGES

* Redesign public interface.

### Features

* Implement Log Exporters ([6bd8c6a](https://github.com/jtwatson/logger/commit/6bd8c6a9c3f412e14db86170d6cf3a71618048f3))


### Documentation

* Update package documentation ([6bd8c6a](https://github.com/jtwatson/logger/commit/6bd8c6a9c3f412e14db86170d6cf3a71618048f3))


### Code Refactoring

* Redesign public interface. ([6bd8c6a](https://github.com/jtwatson/logger/commit/6bd8c6a9c3f412e14db86170d6cf3a71618048f3))
* Rename public methods to fetch the logger ([6bd8c6a](https://github.com/jtwatson/logger/commit/6bd8c6a9c3f412e14db86170d6cf3a71618048f3))

## [0.0.3](https://github.com/jtwatson/logger/compare/v0.0.2...v0.0.3) (2022-11-10)


### Bug Fixes

* Options were not being passed into handler ([#7](https://github.com/jtwatson/logger/issues/7)) ([9271a60](https://github.com/jtwatson/logger/commit/9271a606beb53799d69ac6a11b537d7ac2011a37))

## [0.0.2](https://github.com/jtwatson/logger/compare/v0.0.1...v0.0.2) (2022-11-08)


### Bug Fixes

* Rename GitHub Action to Release ([#4](https://github.com/jtwatson/logger/issues/4)) ([675248b](https://github.com/jtwatson/logger/commit/675248b69653749e44bfd839888ca927824f6bda))

## 0.0.1 (2022-11-08)


### Features

* Initial release ([#1](https://github.com/jtwatson/logger/issues/1)) ([3037efb](https://github.com/jtwatson/logger/commit/3037efb3c03d001a1399a8dab6de0108da701ca6))


### Miscellaneous Chores

* release 0.0.1 ([#3](https://github.com/jtwatson/logger/issues/3)) ([193cac2](https://github.com/jtwatson/logger/commit/193cac249f8f80d3bd360275d4a24391f3c6bcbb))
