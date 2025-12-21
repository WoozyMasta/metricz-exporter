package storage

import "errors"

// ErrStagingFull indicates that the staging buffer has reached its capacity.
var ErrStagingFull = errors.New("staging buffer is full")
