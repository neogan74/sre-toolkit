package notifier

import (
	"context"
	"net/http"

	pkgnotifier "github.com/neogan/sre-toolkit/pkg/notifier"
)

// postJSON delegates to the shared transport.
func postJSON(ctx context.Context, client *http.Client, endpoint string, v any) error {
	return pkgnotifier.PostJSON(ctx, client, endpoint, v)
}
