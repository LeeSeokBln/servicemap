package collect

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

func TestParseHexAddr(t *testing.T) {
	cases := []struct {
		in   string
		ip   string
		port uint16
	}{
		{"0100007F:1F90", "127.0.0.1", 8080},
		{"00000000:0050", "0.0.0.0", 80},
		{"00000000000000000000000001000000:0CEA", "::1", 3306},
		{"00000000000000000000000000000000:0050", "::", 80},
	}
	for _, c := range cases {
		ip, port, err := parseHexAddr(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		if ip != netip.MustParseAddr(c.ip) || port != c.port {
			t.Errorf("%s: got %s:%d, want %s:%d", c.in, ip, port, c.ip, c.port)
		}
	}
	if _, _, err := parseHexAddr("zz:1"); err == nil {
		t.Error("want error for bad hex")
	}
}

func TestReadSockets(t *testing.T) {
	dir := t.TempDir()
	tcp := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:0050 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 101 1 0000000000000000 100 0 0 10 0
   1: 0100007F:CB20 0100007F:0CEA 01 00000000:00000000 00:00000000 00000000     0        0 202 1 0000000000000000 100 0 0 10 0
   2: 0100007F:1234 0100007F:0016 06 00000000:00000000 00:00000000 00000000     0        0 0 1 0000000000000000
`
	udp := `   sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode ref pointer drops
  100: 00000000:0035 00000000:0000 07 00000000:00000000 00:00000000 00000000     0        0 401 2 0000000000000000 0
`
	os.WriteFile(filepath.Join(dir, "tcp"), []byte(tcp), 0o644)
	os.WriteFile(filepath.Join(dir, "udp"), []byte(udp), 0o644)

	socks, err := ReadSockets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(socks) != 4 {
		t.Fatalf("got %d sockets, want 4", len(socks))
	}
	listen := socks[0]
	if listen.State != StateListen || listen.LocalPort != 80 || listen.Proto != "tcp" || listen.Inode != 101 {
		t.Errorf("bad listen socket: %+v", listen)
	}
	est := socks[1]
	if est.State != StateEstablished || est.RemotePort != 3306 ||
		est.RemoteIP != netip.MustParseAddr("127.0.0.1") || est.Inode != 202 {
		t.Errorf("bad established socket: %+v", est)
	}
	if socks[2].State != StateOther {
		t.Errorf("want StateOther for st=06 (TIME_WAIT), got %+v", socks[2])
	}
	if socks[3].State != StateListen || socks[3].Proto != "udp" || socks[3].LocalPort != 53 {
		t.Errorf("bad udp socket: %+v", socks[3])
	}
}

func TestReadSocketsMissingDir(t *testing.T) {
	if _, err := ReadSockets(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("want error when no socket tables exist")
	}
}
