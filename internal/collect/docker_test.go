package collect

import (
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDockerNames(t *testing.T) {
	dir, err := os.MkdirTemp("", "sm") // short path: unix socket paths are length-limited
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	sock := filepath.Join(dir, "docker.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/containers/aaa111/json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"Name":"/webapp"}`))
	})
	mux.HandleFunc("/containers/bbb222/json", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no such container", 404)
	})
	srv := &http.Server{Handler: mux}
	go srv.Serve(l)
	t.Cleanup(func() { srv.Close() })

	names := ResolveDockerNames(sock, []string{"aaa111", "bbb222"})
	if names["aaa111"] != "webapp" {
		t.Errorf(`names["aaa111"] = %q, want "webapp"`, names["aaa111"])
	}
	if _, ok := names["bbb222"]; ok {
		t.Error("404 container must be absent from result")
	}
}

func TestResolveDockerNamesNoSocket(t *testing.T) {
	names := ResolveDockerNames(filepath.Join(t.TempDir(), "missing.sock"), []string{"aaa"})
	if len(names) != 0 {
		t.Errorf("want empty map, got %v", names)
	}
}
