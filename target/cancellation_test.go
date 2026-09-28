package target

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kamaji/rt"
)

func TestDownloadContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	manager := Manager{Runtime: &rt.Runtime{Context: ctx}, Client: &http.Client{Transport: transportFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}}
	err := manager.downloadFile("https://fixture.invalid/artifact?query=PUBLIC_MARKER", filepath.Join(t.TempDir(), "payload"))
	if !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "PUBLIC_MARKER") {
		t.Fatalf("safe cancellation identity lost: %v", err)
	}
}
