// Package config defines the configuration-source abstractions for the
// go-wind framework.
//
// It provides three interfaces that concrete providers (file, env, etcd,
// consul, etc.) implement:
//   - [Reader]   — one-shot key lookups.
//   - [Watcher]  — reactive change notifications.
//   - [ReadWatcher] — combines Reader and Watcher.
package config

import "context"

// Reader provides one-shot loading of configuration data by key.
// It intentionally excludes lifecycle methods — providers that hold
// resources (files, connections) should implement [ReadCloser] instead.
type Reader interface {
	// Load retrieves the raw configuration bytes for the given key.
	Load(ctx context.Context, key string) (data []byte, err error)
}

// Closer releases any resources held by a config provider. It mirrors
// [io.Closer] and is used as a building block for [ReadCloser].
type Closer interface {
	Close() error
}

// ReadCloser combines [Reader] and [Closer] for providers that hold resources
// (files, network connections, etc.) which must be explicitly released.
type ReadCloser interface {
	Reader
	Closer
}

// Watcher provides reactive configuration change notifications.
type Watcher interface {
	// Watch returns a channel that receives a signal each time the value
	// associated with key changes. The channel is closed when the watcher
	// is stopped or ctx is cancelled.
	Watch(ctx context.Context, key string) (<-chan struct{}, error)
}

// ReadWatcher combines [Reader] and [Watcher] for providers that support
// both one-shot reads and reactive updates.
type ReadWatcher interface {
	Reader
	Watcher
}

// ValueWatcher provides push-mode configuration change notifications.
// Unlike [Watcher] which only signals that a value changed (requiring a
// follow-up [Reader.Load] call), ValueWatcher delivers the new value
// directly on the channel. This is an optional interface — providers that
// only support signal-mode notifications implement [Watcher] and leave this
// to those that can efficiently deliver changed values.
type ValueWatcher interface {
	// WatchValue returns a channel that receives the new raw value each
	// time the data associated with key changes. The channel is closed when
	// the watcher is stopped or ctx is cancelled.
	WatchValue(ctx context.Context, key string) (<-chan []byte, error)
}
