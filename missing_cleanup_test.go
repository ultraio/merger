package merger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/streamingfast/dstore"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type cleanupTestStore struct {
	dstore.Store
	failures []error
	calls    int
}

func (s *cleanupTestStore) DeleteObject(context.Context, string) error {
	i := s.calls
	s.calls++
	if i < len(s.failures) {
		return s.failures[i]
	}
	return nil
}
func TestCleanupAlreadyAbsentObject(t *testing.T) {
	for _, test := range []struct {
		name     string
		failures []error
		calls    int
		warnings int
	}{
		{"GCS absent", []error{storage.ErrObjectNotExist}, 1, 0},
		{"wrapped GCS absent", []error{fmt.Errorf("delete: %w", storage.ErrObjectNotExist)}, 1, 0},
		{"store absent", []error{dstore.ErrNotFound}, 1, 0},
		{"local absent", []error{&os.PathError{Op: "remove", Path: "oneblock", Err: os.ErrNotExist}}, 1, 0},
		{"transient then success", []error{errors.New("unavailable")}, 2, 0},
		{"permission denied", []error{os.ErrPermission, os.ErrPermission, os.ErrPermission}, 3, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			previous := zlog
			zlog = zap.New(core)
			defer func() { zlog = previous }()
			store := &cleanupTestStore{failures: test.failures}
			queue := make(chan string, 1)
			queue <- "oneblock"
			close(queue)
			deleter := &oneBlockFilesDeleter{store: store, toProcess: queue}
			deleter.processDeletions()
			require.Equal(t, test.calls, store.calls)
			require.Equal(t, test.warnings, logs.Len())
		})
	}
}
