// Package geoip handles downloading, updating, and reading MaxMind GeoLite2 databases.
package geoip

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/woozymasta/metricz-exporter/internal/config"
)

// EnsureDB checks if the GeoIP database exists at the specified path and if it is recent enough.
// If the file is missing or older than maxAge, it downloads a new copy from the provided URL.
func EnsureDB(cfg config.GeoIPConfig) error {
	info, err := os.Stat(cfg.Path)

	if err == nil {
		if cfg.MaxAge > 0 && time.Since(info.ModTime()) < cfg.MaxAge.ToDuration() {
			log.Info().Str("path", cfg.Path).Msg("GeoIP database is up to date")
			return nil
		}
		log.Info().Str("path", cfg.Path).Msg("GeoIP database is outdated, updating...")
	} else if os.IsNotExist(err) {
		log.Info().Str("path", cfg.Path).Msg("GeoIP database missing, downloading...")
	} else {
		return err
	}

	return downloadFile(cfg.Path, cfg.URL)
}

// downloadFile downloads a file from a URL to a local path using a temporary file
// to ensure atomic writes.
func downloadFile(filepath string, url string) error {
	tmpPath := filepath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Msg("failed to download GeoIP DB")
		return os.ErrInvalid
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	return os.Rename(tmpPath, filepath)
}
