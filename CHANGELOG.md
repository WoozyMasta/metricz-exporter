# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## [0.1.2][] - 2025-12-21

### Added

* new configuration option `max_staging_size` to prevent memory
  exhaustion during chunked ingestion, incomplete transactions now limited
  by total memory usage (default 64MiB).

### Changed

* fixed RCon poller incorrectly overwriting metrics series instead of
  appending them.

[0.1.2]: https://github.com/WoozyMasta/metricz-exporter/compare/v0.1.1...v0.1.2

## [0.1.1][] - 2025-12-16

### Added

* public status metrics with multiple series are now aggregated
  * values are exported as sum of all Gauge/Counter samples
  * labels are exported as unique value lists across all samples
  * single-sample-only limitation removed
* option `extra_labels` for add extra labels for all exported metrics
* option `disable_go_collector` for disable Go runtime metrics
* option `disable_process_collector` for disable process state metrics
* metrics documentation file `METRICS.md`

[0.1.1]: https://github.com/WoozyMasta/metricz-exporter/compare/v0.1.0...v0.1.1

## [0.1.0][] - 2025-12-15

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/metricz-exporter/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
