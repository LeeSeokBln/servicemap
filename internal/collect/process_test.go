package collect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCgroup(t *testing.T) {
	cases := []struct {
		name, in, unit, container string
	}{
		{"systemd v2", "0::/system.slice/nginx.service", "nginx.service", ""},
		{"nested service", "0::/system.slice/system-mariadb.slice/mariadb.service", "mariadb.service", ""},
		{"docker systemd driver", "0::/system.slice/docker-abcdef123456abcdef123456abcdef123456abcdef123456abcdef123456abcd.scope",
			"", "abcdef123456abcdef123456abcdef123456abcdef123456abcdef123456abcd"},
		{"docker cgroupfs driver", "0::/docker/abcdef123456", "", "abcdef123456"},
		{"user session ignored", "0::/user.slice/user-1000.slice/user@1000.service/app.slice/foo.scope", "", ""},
		{"cgroup v1 multiline", "12:pids:/system.slice/redis.service\n1:name=systemd:/system.slice/redis.service\n", "redis.service", ""},
		{"root cgroup", "0::/", "", ""},
	}
	for _, c := range cases {
		unit, container := parseCgroup(c.in)
		if unit != c.unit || container != c.container {
			t.Errorf("%s: got (%q,%q), want (%q,%q)", c.name, unit, container, c.unit, c.container)
		}
	}
}

// writeProc creates a minimal /proc/<pid> under root.
func writeProc(t *testing.T, root string, pid, comm, cgroup, cmdline string, inodes []string) {
	t.Helper()
	dir := filepath.Join(root, "proc", pid)
	if err := os.MkdirAll(filepath.Join(dir, "fd"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "cgroup"), []byte(cgroup+"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644)
	os.MkdirAll(filepath.Join(dir, "ns"), 0o755)
	os.Symlink("net:[4026531840]", filepath.Join(dir, "ns", "net"))
	for i, ino := range inodes {
		os.Symlink("socket:["+ino+"]", filepath.Join(dir, "fd", string(rune('3'+i))))
	}
}

func TestScanProcesses(t *testing.T) {
	root := t.TempDir()
	writeProc(t, root, "100", "nginx", "0::/system.slice/nginx.service",
		"nginx: master process /usr/sbin/nginx\x00", []string{"101", "102"})
	writeProc(t, root, "200", "node", "0::/", "node\x00server.js\x00", []string{"201"})
	// non-numeric entry must be ignored
	os.MkdirAll(filepath.Join(root, "proc", "sys"), 0o755)

	procs, skipped, err := ScanProcesses(root)
	if err != nil {
		t.Fatal(err)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}
	if len(procs) != 2 {
		t.Fatalf("got %d procs, want 2", len(procs))
	}
	nginx := procs[0]
	if nginx.PID != 100 || nginx.Comm != "nginx" || nginx.Unit != "nginx.service" {
		t.Errorf("bad nginx proc: %+v", nginx)
	}
	if len(nginx.SocketInodes) != 2 || nginx.SocketInodes[0] != 101 {
		t.Errorf("bad inodes: %v", nginx.SocketInodes)
	}
	if nginx.NetNS != "net:[4026531840]" {
		t.Errorf("bad netns: %q", nginx.NetNS)
	}
	node := procs[1]
	if node.Unit != "" || len(node.Cmdline) != 2 || node.Cmdline[1] != "server.js" {
		t.Errorf("bad node proc: %+v", node)
	}
}
