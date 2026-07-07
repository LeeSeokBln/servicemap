// Package testfix builds a fake filesystem root mimicking a small Linux
// server: nginx (idle, proxy configured) -> myapp -> mariadb, one docker
// container in its own netns, and a plain client process.
package testfix

import (
	"os"
	"path/filepath"
	"testing"
)

const containerID = "abcdef123456abcdef123456abcdef123456abcdef123456abcdef123456abcd"

// Hex addr notes: port 80=0050, 3000=0BB8, 3306=0CEA, 8080=1F90,
// 52000=CB20, 53248=D000; 127.0.0.1=0100007F (little-endian groups).
const hostTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 101 1 0000000000000000 100 0 0 10 0
   1: 00000000:0BB8 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 201 1 0000000000000000 100 0 0 10 0
   2: 00000000:0CEA 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 301 1 0000000000000000 100 0 0 10 0
   3: 0100007F:CB20 0100007F:0CEA 01 00000000:00000000 00:00000000 00000000     0        0 202 1 0000000000000000 100 0 0 10 0
   4: 0100007F:0CEA 0100007F:CB20 01 00000000:00000000 00:00000000 00000000     0        0 302 1 0000000000000000 100 0 0 10 0
   5: 0100007F:D000 0100007F:0050 01 00000000:00000000 00:00000000 00000000     0        0 501 1 0000000000000000 100 0 0 10 0
   6: 0100007F:0050 0100007F:D000 01 00000000:00000000 00:00000000 00000000     0        0 102 1 0000000000000000 100 0 0 10 0
`

const containerTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 401 1 0000000000000000 100 0 0 10 0
`

const nginxConf = `events {}
http {
    upstream app {
        server 127.0.0.1:3000;
    }
    include conf.d/*.conf;
}
`

const siteConf = `server {
    listen 80;
    location / {
        proxy_pass http://app;
    }
}
`

// WriteRoot creates the fixture and returns its path.
func WriteRoot(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	mustWrite := func(path string, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	proc := func(pid, comm, cgroup, cmdline, netns string, inodes ...string) {
		t.Helper()
		dir := filepath.Join(root, "proc", pid)
		mustWrite(filepath.Join(dir, "comm"), comm+"\n")
		mustWrite(filepath.Join(dir, "cgroup"), cgroup+"\n")
		mustWrite(filepath.Join(dir, "cmdline"), cmdline)
		os.MkdirAll(filepath.Join(dir, "ns"), 0o755)
		if err := os.Symlink(netns, filepath.Join(dir, "ns", "net")); err != nil {
			t.Fatal(err)
		}
		os.MkdirAll(filepath.Join(dir, "fd"), 0o755)
		for i, ino := range inodes {
			if err := os.Symlink("socket:["+ino+"]",
				filepath.Join(dir, "fd", string(rune('3'+i)))); err != nil {
				t.Fatal(err)
			}
		}
	}

	mustWrite(filepath.Join(root, "proc", "net", "tcp"), hostTCP)
	os.MkdirAll(filepath.Join(root, "proc", "1", "ns"), 0o755)
	os.Symlink("net:[1]", filepath.Join(root, "proc", "1", "ns", "net"))

	proc("100", "nginx", "0::/system.slice/nginx.service",
		"nginx: master process /usr/sbin/nginx\x00", "net:[1]", "101", "102")
	proc("200", "myapp", "0::/system.slice/myapp.service",
		"/usr/bin/myapp\x00", "net:[1]", "201", "202")
	proc("300", "mariadbd", "0::/system.slice/mariadb.service",
		"/usr/sbin/mariadbd\x00", "net:[1]", "301", "302")
	proc("400", "webapp", "0::/system.slice/docker-"+containerID+".scope",
		"webapp\x00", "net:[999]", "401")
	proc("500", "cachetool", "0::/",
		"cachetool\x00", "net:[1]", "501")

	mustWrite(filepath.Join(root, "proc", "400", "net", "tcp"), containerTCP)
	mustWrite(filepath.Join(root, "etc", "nginx", "nginx.conf"), nginxConf)
	mustWrite(filepath.Join(root, "etc", "nginx", "conf.d", "site.conf"), siteConf)
	return root
}
