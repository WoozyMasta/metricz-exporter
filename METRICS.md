# Metrics

## MetricZ Ingest

All metrics are exposed with the `instance_id` label,
derived from the ingest route path and the ingested payload.

* **`metricz_ingest_bytes_total`** (`COUNTER`) —
  Total bytes received from the instance via ingest API
* **`metricz_ingest_chunks_total`** (`COUNTER`) —
  Total chunks received from the instance via ingest API
* **`metricz_ingest_last_timestamp_seconds`** (`GAUGE`) —
  Unix timestamp of the last successful ingest
* **`metricz_ingest_transactions_expired_total`** (`COUNTER`) —
  Total chunked transactions dropped due to TTL expiration

## A2S

All metrics are exposed with the `instance_id` label
defined in the configuration.

> [!NOTE]  
> Exposed only when A2S is enabled in the configuration.

* **`metricz_a2s_info`** (`GAUGE`) —
  Static metadata about the game server.  
  Labels:
  * `game_address` - IP from config + Game Port from A2S
  * `query_address` - IP:Port from config
  * `server_name` - Server name
  * `server_description` - Description of game server
  * `version` - Game server version
  * `world` - Map name
  * `environment` - Server OS name
* **`metricz_a2s_info_players_online`** (`GAUGE`) — Online players
* **`metricz_a2s_info_players_queue`** (`GAUGE`) — Players wait in queue
* **`metricz_a2s_info_players_slots`** (`GAUGE`) — Players slots count
* **`metricz_a2s_info_response_time_seconds`** (`GAUGE`) —
  Server A2S_INFO response time
* **`metricz_a2s_up`** (`GAUGE`) —
  A2S server availability (1 = up, 0 = down)

## BattlEye RCon

All metrics are exposed with the `instance_id` label
defined in the configuration.

> [!NOTE]  
> Exposed only when RCon is enabled in the configuration.

* **`metricz_rcon_players_lobby`** (`GAUGE`) —
  Players in lobby (connecting state)
* **`metricz_rcon_players_total`** (`GAUGE`) —
  Total clients connected (including lobby)
* **`metricz_rcon_up`** (`GAUGE`) —
  RCon availability (1 = up, 0 = down)

### Per-player RCon metrics

Each per-player metric includes the `buid` label,
which is the BattlEye GUID hash of the player's SteamID.

* **`metricz_rcon_player_joined`** (`GAUGE`) —
  Player connection state
  (0 = lobby, loading, or queued; 1 = actively playing).  
  Labels:
  * `ip` - Player IP address
  * `country` - Shot country code name (e.g. US, DE, RU, FR), _if GeoIP enabled_
  * `name` - Player Name
* **`metricz_rcon_player_lat`** (`GAUGE`) —
  Player Latitude, _if GeoIP enabled_
* **`metricz_rcon_player_lon`** (`GAUGE`) —
  Player Longitude, _if GeoIP enabled_
* **`metricz_rcon_player_ping_seconds`** (`GAUGE`) —
  Player latency

### MetricZ Injection

For the ingested metric `dayz_metricz_player_loaded`,
the `buid` label is injected automatically.

## System Metrics

The exporter also exposes framework-level system metrics:
`go_*` and `process_*`, including exporter runtime information.
