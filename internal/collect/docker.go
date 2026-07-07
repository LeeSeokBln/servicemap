package collect

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// ResolveDockerNames maps container IDs to names via the docker API socket.
// Best-effort: on any error the affected IDs are simply absent.
func ResolveDockerNames(sockPath string, ids []string) map[string]string {
	names := map[string]string{}
	info, err := os.Stat(sockPath)
	if err != nil || info.Mode()&os.ModeSocket == 0 {
		return names
	}
	// A fresh client per call is fine: this runs once per collection and the
	// DialContext closure must capture sockPath.
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", sockPath)
			},
		},
	}
	for _, id := range ids {
		resp, err := client.Get("http://docker/containers/" + id + "/json")
		if err != nil {
			return names // daemon unreachable: stop trying
		}
		var doc struct {
			Name string `json:"Name"`
		}
		decErr := json.NewDecoder(resp.Body).Decode(&doc)
		resp.Body.Close()
		if decErr != nil || resp.StatusCode != http.StatusOK || doc.Name == "" {
			continue
		}
		names[id] = strings.TrimPrefix(doc.Name, "/")
	}
	return names
}
