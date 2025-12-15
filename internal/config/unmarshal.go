package config

import (
	"encoding/json"
	"errors"
	"time"
)

// Duration is a wrapper around time.Duration that supports JSON string unmarshaling
// (e.g. "15s", "1m") which standard encoding/json does not support for time.Duration.
type Duration time.Duration

// UnmarshalJSON implements the json.Unmarshaler interface.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil

	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil

	default:
		return errors.New("invalid duration")
	}
}

// MarshalJSON implements the json.Marshaler interface.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalText implements encoding.TextUnmarshaler.
// This is required for the 'defaults' library to parse default tag strings like "15s".
func (d *Duration) UnmarshalText(text []byte) error {
	tmp, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}

	*d = Duration(tmp)
	return nil
}

// ToDuration allows easy casting back to standard time.Duration if explicit cast is annoying.
func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}
