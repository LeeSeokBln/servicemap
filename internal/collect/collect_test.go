package collect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollect(t *testing.T) {
	root := t.TempDir()
	netDir := filepath.Join(root, "proc", "net")
	os.MkdirAll(netDir, 0o755)
	hostTCP := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 101 1 0000000000000000 100 0 0 10 0
`
	os.WriteFile(filepath.Join(netDir, "tcp"), []byte(hostTCP), 0o644)

	// host nginx process (pid 100) in host netns
	writeProc(t, root, "100", "nginx", "0::/system.slice/nginx.service",
		"nginx: master process /usr/sbin/nginx\x00", []string{"101"})

	// container process (pid 400) in its own netns with its own socket table
	writeProc(t, root, "400", "webapp",
		"0::/system.slice/docker-abcdef123456abcdef123456abcdef123456abcdef123456abcdef123456abcd.scope",
		"webapp\x00", []string{"401"})
	os.Remove(filepath.Join(root, "proc", "400", "ns", "net"))
	os.Symlink("net:[999]", filepath.Join(root, "proc", "400", "ns", "net"))
	contNet := filepath.Join(root, "proc", "400", "net")
	os.MkdirAll(contNet, 0o755)
	contTCP := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 401 1 0000000000000000 100 0 0 10 0
`
	os.WriteFile(filepath.Join(contNet, "tcp"), []byte(contTCP), 0o644)

	// pid 1 netns marker (same value as writeProc default = host ns)
	os.MkdirAll(filepath.Join(root, "proc", "1", "ns"), 0o755)
	os.Symlink("net:[4026531840]", filepath.Join(root, "proc", "1", "ns", "net"))

	// nginx config
	confDir := filepath.Join(root, "etc", "nginx")
	os.MkdirAll(confDir, 0o755)
	os.WriteFile(filepath.Join(confDir, "nginx.conf"),
		[]byte("http { proxy_pass http://127.0.0.1:3000; }\n"), 0o644)

	snap, err := Collect(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Processes) != 2 {
		t.Fatalf("got %d processes, want 2", len(snap.Processes))
	}
	if len(snap.Sockets) != 2 {
		t.Fatalf("got %d sockets (host+container), want 2: %+v", len(snap.Sockets), snap.Sockets)
	}
	var found8080 bool
	for _, s := range snap.Sockets {
		if s.Inode == 401 && s.LocalPort == 8080 {
			found8080 = true
		}
	}
	if !found8080 {
		t.Error("container netns socket (inode 401, :8080) not collected")
	}
	if snap.Nginx == nil || len(snap.Nginx.ProxyPass) != 1 {
		t.Errorf("nginx config not parsed: %+v", snap.Nginx)
	}
	if snap.SkippedPIDs != 0 {
		t.Errorf("SkippedPIDs = %d", snap.SkippedPIDs)
	}
}

func TestNginxConfigPathFromCmdline(t *testing.T) {
	root := t.TempDir()
	procs := []Process{{
		PID: 5, Comm: "nginx",
		Cmdline: []string{"nginx: master process /usr/sbin/nginx -c /opt/nginx/custom.conf"},
	}}
	got := nginxConfigPath(root, procs)
	want := filepath.Join(root, "opt", "nginx", "custom.conf")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNginxConfigPathNoNginx(t *testing.T) {
	if got := nginxConfigPath(t.TempDir(), []Process{{PID: 1, Comm: "systemd"}}); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
