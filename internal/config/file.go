package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/creasty/defaults"
	"github.com/prometheus/common/model"
	"github.com/woozymasta/jamle"
	"github.com/woozymasta/metricz-exporter/internal/logger"
)

// Config is the root configuration object loaded from YAML/JSON.
type Config struct {
	// PublicExport configures what /status (public endpoints) exports and how it filters labels.
	PublicExport PublicExportConfig `json:"public_export"`

	// Servers lists monitored/ingested instances (A2S, RCon, etc.).
	Servers []ServerDefinition `json:"servers"`

	// App contains exporter runtime settings (HTTP server, auth, ingest, etc.).
	App AppConfig `json:"exporter"`
}

// AppConfig groups top-level exporter behavior.
type AppConfig struct {
	// Prometheus congiguration
	Prometheus PrometheusConfig `json:"prometheus"`

	// Logger config is applied after load+validate.
	Logger logger.Logger `json:"log"`

	// Auth is used for private ingest endpoints (/ingest, /commit).
	Auth AuthConfig `json:"auth"`

	// ListenAddr is the TCP address the exporter listens on.
	ListenAddr string `json:"listen_addr" default:":8098"`

	// GeoIP optionally enriches data with GeoIP database.
	GeoIP GeoIPConfig `json:"geo_ip"`

	// Ingest limits and behavior for ingest endpoints (body size, chunk GC, etc.).
	Ingest IngestConfig `json:"ingest"`

	// Public affects public endpoints behavior (currently cache TTL and CORS).
	Public PublicConfig `json:"public"`

	// Stale config defines when a server/metrics are considered stale/down.
	Stale StaleConfig `json:"stale"`
}

// AuthConfig controls Basic Auth for private endpoints.
type AuthConfig struct {
	// User is Basic Auth username for ingest endpoints.
	User string `json:"user" default:"metricz"`

	// Pass is Basic Auth password for ingest endpoints.
	// No default: forcing explicit config is intentional.
	Pass string `json:"password"`
}

// PublicConfig controls public endpoints behavior.
type PublicConfig struct {
	// PublicCacheTTL is TTL for cached /status responses (or underlying cached payload).
	PublicCacheTTL Duration `json:"cache_ttl" default:"15s"`

	// Enable /status endpoints
	Enabled bool `json:"enabled"`

	// PublicCORS enables "Access-Control-Allow-Origin: *" for public JSON endpoints.
	PublicCORS bool `json:"cors"`
}

// IngestConfig controls ingest request lifecycle and safety limits.
type IngestConfig struct {
	// TransactionTTL is TTL for incomplete chunked uploads; expired transactions are dropped.
	// Applies to /ingest/{instance_id}/{txn_hash}/{seq_id} + /commit.
	TransactionTTL Duration `json:"transaction_ttl" default:"15s"`

	// GarbageCollectorTTL controls how often expired transactions are cleaned up.
	GarbageCollectorTTL Duration `json:"gc_ttl" default:"60s"`

	// MaxBodySize is max HTTP request body in bytes (hard limit).
	MaxBodySize int64 `json:"max_body_size" default:"4194304"` // 4 MiB

	// MaxStagingSize is the maximum allowed memory usage (in bytes) for incomplete transactions.
	MaxStagingSize int64 `json:"max_staging_size" default:"67108864"` // 64 MiB

	// OverwriteInstanceID allows ingest payload to override instance_id label even
	// if it differs from instance_id in URL.
	OverwriteInstanceID bool `json:"overwrite_instance_id"`
}

// StaleConfig controls "staleness" detection.
type StaleConfig struct {
	// StaleMultiplier multiplies scrape/poll interval to decide "down".
	// Example: poll_interval=15s, multiplier=2.0 => mark stale after ~30s since last update.
	StaleMultiplier float64 `json:"multiplier" default:"2.0"`

	// MinStaleAge is the lower bound for stale marking regardless of multiplier.
	MinStaleAge Duration `json:"min_age" default:"30s"`
}

// GeoIPConfig points to GeoLite2/GeoIP2 database.
type GeoIPConfig struct {
	// Path is a path to *.mmdb database.
	Path string `json:"path" default:"metricz-city.mmdb"`

	// URL to download MMDB.
	URL string `json:"url" default:"https://git.io/GeoLite2-City.mmdb"`

	// MaxAge is a max age of file.
	MaxAge Duration `json:"max_age" default:"24h"`
}

// PrometheusConfig extra prometheus settings.
type PrometheusConfig struct {
	// ConstantLabels are added to every metric exposed by this exporter.
	// WARNING: changing labels creates new time series.
	ExtraLabels map[string]string `json:"extra_labels"`

	// Disable the collector that exports metrics about the current Go process and runtime
	DisableGoCollector bool `json:"disable_go_collector"`

	// Disables the collector that exports metrics about the current state of the process, including
	// CPU, memory, and file descriptor usage, as well as the process startup time.
	DisableProcessCollector bool `json:"disable_process_collector"`
}

// PublicExportConfig configures /status output.
type PublicExportConfig struct {
	// Values is an allowlist of metric names whose samples/values are included in /status output.
	Values []string `json:"values" default:"[\"dayz_metricz_status\", \"metricz_a2s_info\"]"`

	// Labels is an allowlist of metric names for which labels are exported in /status output.
	Labels []string `json:"labels" default:"[\"dayz_metricz_status\", \"metricz_a2s_info\"]"`

	// LabelsExclude is a denylist of label keys removed from exported labels.
	LabelsExclude []string `json:"labels_exclude" default:"[\"steam_id\", \"guid\", \"buid\", \"name\", \"ip\", \"city\", \"country\"]"`
}

// ServerDefinition describes one logical instance (instance_id) and its data sources.
type ServerDefinition struct {
	// A2S is optional. If non-nil, exporter will poll A2S endpoint for that instance.
	A2S *A2SConfig `json:"a2s,omitempty"`

	// RCon is optional. If non-nil, exporter will connect to RCon endpoint for that instance.
	RCon *RConConfig `json:"rcon,omitempty"`

	// InstanceID is the stable logical id used in URLs and labels.
	// Must be unique and non-empty.
	InstanceID string `json:"instance_id"`
}

// A2SConfig configures A2S polling.
type A2SConfig struct {
	// Address is "host:port" of the A2S server.
	Address string `json:"address"`

	// PoolInterval is polling interval for A2S data collection.
	PoolInterval Duration `json:"poll_interval" default:"15s"`

	// DeadlineTimeout is per-request deadline for A2S operations.
	// Keeps polling from hanging on slow/broken networks.
	DeadlineTimeout Duration `json:"deadline_timeout" default:"5s"`

	// BufferSize is UDP buffer size (or read buffer) used by A2S implementation.
	// Keep aligned with protocol payload sizes; too small => truncation; too large => memory overhead.
	BufferSize uint16 `json:"buffer_size" default:"1400"`
}

// RConConfig configures RCon polling/connection.
type RConConfig struct {
	// Address is "host:port" of RCon endpoint.
	Address string `json:"address"`

	// Password is RCon password (secret).
	Password string `json:"password"`

	// PoolInterval is interval between RCon queries/commands used for metrics extraction.
	PoolInterval Duration `json:"poll_interval" default:"15s"`

	// KeepaliveTimeout controls how long to keep idle connection open (if implementation supports it).
	KeepaliveTimeout Duration `json:"keepalive_timeout" default:"30s"`

	// DeadlineTimeout is per-operation deadline (dial/read/write).
	DeadlineTimeout Duration `json:"deadline_timeout" default:"5s"`

	// BufferSize is read buffer for RCon packets.
	BufferSize uint16 `json:"buffer_size" default:"1024"`

	// LoginAttempts Number of login attempts.
	LoginAttempts int `json:"login_attempts" default:"1"`
}

// LoadConfig reads config from path (YAML/JSON), applies defaults, validates, and configures logger.
func LoadConfig(path string) (*Config, error) {
	cfg := new(Config)

	_, err := os.Stat(path)
	if err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		if err := jamle.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to stat config file: %w", err)
	}

	// Apply defaults after parsing so config file overrides defaults
	if err := defaults.Set(cfg); err != nil {
		return nil, err
	}

	// Validate logical constraints and required secrets
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	// Apply logger configuration after validation
	cfg.App.Logger.Setup()

	return cfg, nil
}

// validate checks configuration for logical errors
func (cfg *Config) validate() error {
	seenIDs := make(map[string]bool)

	for i := range cfg.Servers {
		srv := &cfg.Servers[i]

		if srv.InstanceID == "" {
			return fmt.Errorf("server at index %d has empty instance_id", i)
		}

		if seenIDs[srv.InstanceID] {
			return fmt.Errorf("duplicate instance_id found: '%s'", srv.InstanceID)
		}
		seenIDs[srv.InstanceID] = true

		if srv.A2S != nil {
			if srv.A2S.Address == "" {
				return fmt.Errorf("instance '%s': a2s enabled but address is empty", srv.InstanceID)
			}
		}

		if srv.RCon != nil {
			if srv.RCon.Address == "" {
				return fmt.Errorf("instance '%s': rcon enabled but address is empty", srv.InstanceID)
			}
			if srv.RCon.Password == "" {
				return fmt.Errorf("instance '%s': rcon enabled but password is empty", srv.InstanceID)
			}
		}
	}

	if err := validateExtraLabels(cfg.App.Prometheus.ExtraLabels); err != nil {
		return err
	}

	return nil
}

// validateExtraLabels
func validateExtraLabels(m map[string]string) error {
	if len(m) == 0 {
		return nil
	}

	deny := map[string]struct{}{
		"job": {}, "instance": {}, "le": {}, "quantile": {},
	}

	for k, v := range m {
		if k == "" || v == "" {
			return fmt.Errorf("prometheus.extra_labels: empty key/value")
		}
		if _, ok := deny[k]; ok {
			return fmt.Errorf("prometheus.extra_labels: reserved label %q", k)
		}
		if strings.HasPrefix(k, "__") {
			return fmt.Errorf("prometheus.extra_labels: reserved label prefix %q", k)
		}
		if !model.ValidationScheme.IsValidLabelName(model.UTF8Validation, k) {
			return fmt.Errorf("prometheus.extra_labels: invalid label name %q", k)
		}
	}

	return nil
}
