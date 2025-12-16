# MetricZ Exporter

<!-- markdownlint-disable-next-line MD033 -->
<img src="logo.png" alt="MetricZ" align="right" width="300">

Prometheus exporter designed for the [MetricZ] DayZ mod.
It serves as a high-performance backend written in Go,
decoupling metric aggregation from the game server's simulation loop.

This service acts as an intelligent bridge:
it accepts raw telemetry pushed by the game server mod (via HTTP),
buffers it, and exposes the aggregated metrics for Prometheus scraping.  
Unlike simple textfile collection, the exporter provides data enrichment
and external integration features:

* **Steam A2S Integration:**
  Fetches real-time query data, including server name, version,
  and the player queue length.
* **BattlEye RCon & GeoIP:**
  Connects via RCon to correlate player identities with their IP addresses
  and real-world GeoIP location.
* **Public Status API:** Offers a lightweight, cacheable JSON endpoint
  (`/status`), perfect for displaying live server stats on your website
  without exposing your monitoring stack.
* **Transaction Support:** Handles chunked uploads from the mod to minimize
  network overhead on the game server.

A detailed description of additional metrics is described in the
[METRICS.md](./METRICS.md) document.

<!-- markdownlint-disable-next-line MD033 -->
## Install <br clear="right"/>

Download binary from [releases page] or build it,
just run `make build` or use container image.

## Container

Container images are available at:

* `ghcr.io/woozymasta/metricz-exporter:latest`
* `docker.io/woozymasta/metricz-exporter:latest`

```bash
mkdir -p metricz
docker run -d --name metricz-exporter \
  -p 8098:8098 \
  -v "$PWD/metricz:/metricz" \
  ghcr.io/woozymasta/metricz-exporter:latest
```

## Configuration

The application is configured via a YAML file.
You can generate a default configuration file by running:

```bash
./metricz-exporter --init-config
```

<!-- markdownlint-disable-next-line MD033 -->
<details><summary>Click to expand default configuration</summary>

### Configuration File

<!-- include:start -->
```yaml
# metricz-exporter configuration file example (YAML)
#
# Notes:
# - Durations use Go time.Duration. Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
# - Secrets (passwords) should not be committed. Use secret injection
#
# Security model:
# - Private endpoints: /api/v1/ingest/*, /api/v1/commit/*, and /metrics are protected by BasicAuth
#   ONLY when BOTH exporter.auth.user and exporter.auth.password are non-empty
#   If either is empty -> auth is DISABLED and these endpoints become unauthenticated
# - Public endpoints: /api/v1/status* and /health* are always unauthenticated in-app
#
# /api/v1/status export caveats:
# - values/labels export uses ONLY the first sample of each metric family (Metric[0])
#   Multi-series metrics (same name, different labels) are truncated
#
# Configuration supports inline Environment Variables:
# - ${VAR}            Replaces with the value of `VAR`
# - ${VAR:-default}   Uses `default` if `VAR` is unset or empty
# - ${VAR:?error}     Fails to start with an error message if `VAR` is unset
# - $${VAR}           Escaping. Evaluates to the literal string ${VAR} without expansion

---
# Base exporter configuration
exporter:
  # Address to listen on (bind address)
  # - Use "127.0.0.1:8098" if you put a reverse proxy in front
  # - Use ":8098" / "0.0.0.0:8098" only with firewall/reverse-proxy and enabled auth
  listen_addr: ${METRICZ_LISTEN_ADDRESS:-:8098} # (8098 by default)

  # Basic Auth for private endpoints:
  # - POST /api/v1/ingest/{instance_id}
  # - POST /api/v1/ingest/{instance_id}/{txn_hash}/{seq_id}
  # - POST /api/v1/commit/{instance_id}/{txn_hash}
  # - GET  /metrics
  #
  # Enable rule:
  # - Auth ENABLED only when BOTH user != "" AND password != ""
  # - If either is empty -> auth is DISABLED for ALL endpoints above
  auth:
    # Username (non-empty required to enable auth)
    user: ${METRICZ_AUTH_USER:-metricz} # (metricz by default)

    # Password (secret; non-empty required to enable auth)
    # Leave empty ONLY if you intentionally want unauthenticated ingest/commit/metrics
    password: ${METRICZ_AUTH_PASSWORD:-}

  # Public /status endpoints return selected metrics values/labels in JSON format
  # - /api/v1/status
  # - /api/v1/status/{instance_id}
  #
  # Behavior:
  # - Responses are cached in-memory per key ("all" or {instance_id}) for cache_ttl
  # - Header X-Cache: HIT/MISS
  # - "cors: true" adds Access-Control-Allow-Origin: * ONLY for /status endpoints
  public:
    # Enable /status endpoints
    enabled: ${METRICZ_PUBLIC_ENABLED:-false} # (false by default)

    # Cache TTL for public JSON endpoints (in-memory)
    cache_ttl: ${METRICZ_PUBLIC_CACHE_TTL:-15s} # (15s by default)

    # Enable CORS for public JSON endpoints by setting Allow-Origin: *
    cors: ${METRICZ_PUBLIC_CORS:-false} # (false by default)

  # /api/v1/ingest and /api/v1/commit settings for receiving metrics from clients
  ingest:
    # TTL for incomplete chunked uploads (transaction-based ingest)
    #
    # Effective staging TTL is dynamic:
    # - ttl = max(transaction_ttl, known scrape_interval_seconds for this instance at txn start)
    #   scrape_interval_seconds is learned from the last successful ingest
    #   (metric: dayz_metricz_scrape_interval_seconds), default is 60s if unknown
    #
    # Also:
    # - Starting a new txn for the same instance_id drops any previous unfinished txn of that instance
    transaction_ttl: ${METRICZ_INGEST_TRANSACTION_TTL:-15s} # (15s by default)

    # How often to run garbage collection for expired transactions
    # Note: cleanup happens on GC ticks after ExpiresAt is reached
    gc_ttl: ${METRICZ_INGEST_GC_TTL:-60s} # (60s by default)

    # Max HTTP body size in bytes (applies per request):
    # - single-shot ingest: whole request body
    # - chunked ingest: EACH chunk request body
    max_body_size: ${METRICZ_INGEST_MAX_BODY_SIZE:-4194304} # (4194304 by default)

    # If true, allow payload to override/replace instance_id label when it differs from URL
    # Enabling this allows cross-instance contamination unless you enforce external ACL
    overwrite_instance_id: ${METRICZ_INGEST_OVERWRITE_INSTANCE_ID:-false} # (false by default)

  # Ingest staleness detection (applies to ingested(push) metrics only)
  # Source interval:
  # - Uses dayz_metricz_scrape_interval_seconds from last ingest payload (default 60s if missing)
  #
  # Threshold:
  # - threshold = max(scrape_interval_seconds * multiplier, min_age)
  #
  # When stale:
  # - ingested metric families are suppressed (not exported),
  # - if cached dayz_metricz_status exists, it is exported with value forced to 0
  stale:
    multiplier: ${METRICZ_STALE_MULTIPLIER:-2.0} # (2.0 by default)
    min_age: ${METRICZ_STALE_MIN_AGE:-30s} # (30s by default)

  # GeoIP settings
  geo_ip:
    # Path to GeoLite2/GeoIP2 mmdb database
    # Empty => GeoIP enrichment disabled
    path: ${METRICZ_GEOIP_PATH:-metricz-city.mmdb} # (metricz-city.mmdb by default)

    # URL for download MaxMind MMDB GeoDB
    # Empty => GeoIP downloading/updating disabled
    url: ${METRICZ_GEOIP_URL:-https://git.io/GeoLite2-City.mmdb} # (https://git.io/GeoLite2-City.mmdb by default)

    # Download new file on exporter start if file age greater than this age
    # 0 => disable updates
    max_age: ${METRICZ_GEOIP_MAX_AGE:-24h} # (24h by default)

  # Prometheus extra settings
  prometheus:
    # Disables collector that exports metrics about the current Go process and runtime (go_* prefixed metrics)
    disable_go_collector: ${METRICZ_PROMETHEUS_DISABLE_GO_COLLECTOR:-false} # (false by default)

    # Disables the collector that exports metrics about the current state of the process, including
    # CPU, memory, and file descriptor usage, as well as the process startup time (process_* prefixed metrics)
    disable_process_collector: ${METRICZ_PROMETHEUS_DISABLE_PROCESS_COLLECTOR:-false} # (false by default)

    # ConstantLabels are added to every metric exposed by this exporter.
    # WARNING: changing labels creates new time series.
    extra_labels: {}
      # exporter: metricz
      # datacenter: eu-2

  # Rules for converting raw game world coordinates
  # (X/Z in values or labels) into geographic coordinates (longitude/latitude)
  # on the exporter side to reduce load on the game server
  geo_transform:
    # Toggles geo coordinate transformation on the exporter side
    enabled: ${METRICZ_GEO_TRANSFORM_ENABLED:-false} # (false by default)

    # Metric name that provides effective world size used as the base for coordinate normalization.
    world_size_metric: dayz_metricz_effective_world_size

    # Lists metric names where the X/Y world coordinate is stored in the metric VALUE (Gauge).
    # X is converted to longitude
    # Z is converted to latitude
    metrics_value_x: [dayz_metricz_player_position_x, dayz_metricz_transport_position_x]
    metrics_value_z: [dayz_metricz_player_position_z, dayz_metricz_transport_position_z]

    # Lists metric names where coordinates are stored in LABELS instead of values.
    metrics_label_targets: [dayz_metricz_player_position_z, dayz_metricz_transport_position_z]

    # Label key that contains raw X/Y coordinates
    metrics_label_x_name: longitude
    metrics_label_z_name: latitude

  log:
    # Logging level: trace, debug, info, warn, error, fatal
    level: ${METRICZ_LOG_LEVEL:-info} # (info by default)

    # Format: text or json
    # - text format may use ANSI colors when supported (NO_COLOR disables, FORCE_COLOR=true enables)
    format: ${METRICZ_LOG_FORMAT:-text} # (text by default)

    # Output: stdout, stderr, or a file path
    # If file path: opened with 0644 and append mode; directory must exist
    output: ${METRICZ_LOG_OUTPUT:-stderr} # (stderr by default)

# Server instances for active polling/enrichment (optional)
# /ingest accepts any instance_id by default even if not listed here
#
# However, if instance_id is explicitly specified here, it allows you to enrich the metrics with data:
# - From A2S, such as player queue, server name, and monitoring server availability in active mode
#   Expose `metricz_a2s_*` metrics
# - From BERcon, with a list of players with their IP addresses and geolocations,
#   and the `dayz_metricz_player_loaded` metric from MetricZ will be enriched with the BattlEye GUID
#   Expose `metricz_rcon_*` metrics
servers:
  # One entry per logical instance_id
  # instance_id must be unique across the list and match instance_id label from MetricZ
  - instance_id: "1"
    # Steam A2S_INFO query
    a2s:
      # A2S endpoint host:port
      # It's best to indicate the public IP address here
      address: 127.0.0.1:27016

      # How often to poll A2S (should be >= deadline_timeout)
      poll_interval: 15s # (by default)

      # Per-request deadline (avoid hanging reads)
      deadline_timeout: 5s # (by default)

      # Network buffer size for reads (too small => truncation)
      buffer_size: 1400 # (by default)

    # BattleEye RCon query
    rcon:
      # RCon endpoint host:port
      address: 127.0.0.1:2025

      # RCon password (secret)
      password: ${METRICZ_RCON_PASSWORD:?Need setup RCon connection or not use it!}

      # How often to poll metrics via RCon
      poll_interval: 15s # (by default)

      # Keepalive timeout for idle connections (not more than 45s)
      keepalive_timeout: 30s # (by default)

      # Per-operation deadline
      deadline_timeout: 5s # (by default)

      # Buffer size for RCon packets
      buffer_size: 1024 # (by default)

  - instance_id: "${METRICZ_SERVER_2_INSTANCE_ID:-2}"
    a2s:
      address: ${METRICZ_SERVER_2_A2S_ADDRESS:-127.0.0.1:27017}

    rcon:
      address: ${METRICZ_SERVER_2_RCON_ADDRESS:-127.0.0.1:2125}
      password: ${METRICZ_SERVER_2_RCON_PASSWORD:-}

# public_export config controls what /api/v1/status and /api/v1/status/{instance_id} returns
#
# Behavior:
# - For each metric name, /status aggregates ALL samples in instance_id metrics family.
# - Exported numeric value is SUM of all Gauge/Counter samples in the family.
# - Summary/Histogram/Untyped are ignored.
# - Labels are exported as "label_key -> [values...]" collected from ALL samples in the family.
#   Values are deduplicated and sorted for stable output.
# - LabelsExclude is applied to labels before exporting.
public_export:
  # Metric values allowlist (name -> numeric sum of all samples)
  # [dayz_metricz_status, metricz_a2s_info] (by default)
  values:
    - dayz_metricz_status
    - dayz_metricz_fps_window_avg
    - dayz_metricz_time_of_day
    - metricz_a2s_info
    - metricz_a2s_info_players_online
    - metricz_a2s_info_players_queue
    - metricz_a2s_info_players_slots
  # Metric labels allowlist (name -> {label_key: [values...]})
  labels: # (by default)
    - dayz_metricz_status
    - metricz_a2s_info

  # Denylist of label keys removed from output
  # For security exclude identifiers and personal data
  labels_exclude: [steam_id, guid, buid, name, ip, city, country] # (by default)
```
<!-- include:end -->

<!-- markdownlint-disable-next-line MD033 -->
</details>

## Endpoints

### Public

* `GET /api/v1/status` - Returns selected metrics in JSON format
  (e.g. for website status widgets).
* `GET /api/v1/status/{instance_id}` - Returns status for a specific instance.
* `GET /health` and `GET /health/liveness` - Liveness probe.
* `GET /health/readiness` - Readiness probe.

### Prometheus (Internal)

* `GET /metrics` - Exposes metrics in Prometheus format.

### Ingest (Internal)

Used by the DayZ mod to push data.

* `POST /api/v1/ingest/{instance_id}`
* `POST /api/v1/ingest/{instance_id}/{txn_hash}/{seq_id}`
* `POST /api/v1/commit/{txn_hash}`

## Install with Systemd

You can `ctrl+c/v`

```bash
# install app
sudo curl -sSfLo /usr/local/bin/metricz-exporter \
  https://github.com/WoozyMasta/metricz-exporter/releases/latest/download/metricz-exporter-linux-amd64
sudo chmod +x /usr/local/bin/metricz-exporter

# check it works
metricz-exporter --version

# install systemd service
sudo curl -sSfLo /etc/systemd/system/metricz-exporter.service \
  https://raw.githubusercontent.com/WoozyMasta/metricz-exporter/master/metricz-exporter.service
sudo systemctl daemon-reload

# add system user and group
sudo groupadd --system metricz
sudo useradd --system \
  --gid metricz \
  --home /var/lib/metricz \
  --shell /usr/sbin/nologin \
  metricz

# set permissions
mkdir -p /var/lib/metricz-exporter
sudo chown -R metricz:metricz /var/lib/metricz-exporter
sudo chmod 0600 /var/lib/metricz-exporter

# generate default config
sudo metricz-exporter --print-config > /etc/metricz-exporter.yaml
sudo chown metricz:metricz /etc/metricz-exporter.yaml
sudo chmod 0600 /etc/metricz-exporter.yaml

# edit settings
sudo editor /etc/metricz-exporter.yaml

# start, check and enable
sudo systemctl start metricz-exporter.service
sudo systemctl status metricz-exporter.service
sudo systemctl enable metricz-exporter.service
```

## Install as Windows Service

The application supports native Windows Service execution.
You can register it using the built-in `sc.exe` tool.
Run `cmd.exe` or PowerShell as **Administrator**:

```bat
:: Create the service (note the space after binPath=)
:: Ensure you provide absolute paths to both the executable and the config file.
sc create "MetricZExporter" binPath= "C:\MetricZ\metricz-exporter.exe --config C:\MetricZ\config.yaml" start= auto DisplayName= "MetricZ Exporter"

:: Start the service
sc start "MetricZExporter"

:: Query status
sc query "MetricZExporter"

:: To remove the service later:
:: sc stop "MetricZExporter"
:: sc delete "MetricZExporter"
```

## Prometheus Configuration

Add the following job to your `prometheus.yml`:

```yaml
- job_name: metricz-exporter
  scrape_interval: 5s
  static_configs:
    - targets:
        - 127.0.0.1:8098
```

### With Basic Auth

If you have enabled Basic Authentication,
provide the credentials using `basic_auth`:

```yaml
- job_name: metricz-exporter
  scrape_interval: 5s
  static_configs:
    - targets:
        - 127.0.0.1:8098
  basic_auth:
    username: metricz
    password: $tr0ng
```

### Migration (preserve existing time series)

Time series identity is defined by the metric name and its full label set.
If you previously used the node-exporter or windows-exporter textfile
collector and want to avoid creating new series,
keep the same `job` label and the same `instance` label.

> [!NOTE]  
> `job_name` must remain unique (this is required by vmagent).

Example: scrape metricz-exporter, but preserve
`job="node-exporter"` and `instance="127.0.0.1:9100"`:

```yaml
- job_name: metricz-exporter
  scrape_interval: 5s
  static_configs:
    - targets:
        - 127.0.0.1:8098
  relabel_configs:
    - target_label: job
      replacement: node-exporter
    - target_label: instance
      replacement: 127.0.0.1:9100
```

Replace `node-exporter` and `127.0.0.1:9100`
with the exact instance value you had before.

> [!CAUTION]  
> This configuration may cause overlapping or mixed metrics
> with those exposed by **node_exporter** or **windows_exporter**
> (for example, Go runtime or process-level metrics).
>
> To avoid this, it is recommended to disable these collectors
> in the application itself by setting:
>
> * `exporter.prometheus.disable_go_collector`: `true`
> * `exporter.prometheus.disable_process_collector`: `true`
>
> This will result in some loss of low-level runtime metrics,
> but it allows you to preserve the original time series
> during migration without metric collisions.
>
> Whether to use `relabel_configs` depends on your setup
> and your need for strict time series continuity.

## 👉 [Support Me](https://gist.github.com/WoozyMasta/7b0cabb538236b7307002c1fbc2d94ea)

<!-- links -->
[MetricZ]: https://github.com/WoozyMasta/metricz
[releases page]: https://github.com/woozymasta/metricz-exporter/releases
