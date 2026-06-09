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
type Reader interface {
	// Load retrieves the raw configuration bytes for the given key.
	Load(ctx context.Context, key string) (data []byte, err error)
	// Close releases any resources held by the reader.
	Close() error
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
