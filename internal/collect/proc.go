// Package collect reads /proc, the docker socket, and nginx configs into a
// Snapshot of the host's processes, sockets, and service configuration.
package collect

import (
	"encoding/hex"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type SockState int

const (
	StateOther SockState = iota
	StateListen
	StateEstablished
)

type Socket struct {
	Proto      string // "tcp" or "udp"
	LocalIP    netip.Addr
	LocalPort  uint16
	RemoteIP   netip.Addr
	RemotePort uint16
	State      SockState
	Inode      uint64
}

// ReadSockets parses the socket tables (tcp, tcp6, udp, udp6) in netDir,
// e.g. /proc/net or /proc/<pid>/net for a non-host network namespace.
func ReadSockets(netDir string) ([]Socket, error) {
	files := []struct{ name, proto string }{
		{"tcp", "tcp"}, {"tcp6", "tcp"}, {"udp", "udp"}, {"udp6", "udp"},
	}
	var out []Socket
	found := false
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(netDir, f.name))
		if err != nil {
			continue
		}
		found = true
		for _, line := range strings.Split(string(data), "\n") {
			if s, ok := parseSockLine(line, f.proto); ok {
				out = append(out, s)
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("no socket tables under %s", netDir)
	}
	return out, nil
}

func parseSockLine(line, proto string) (Socket, bool) {
	f := strings.Fields(line)
	if len(f) < 10 || !strings.Contains(f[1], ":") {
		return Socket{}, false // header or blank line
	}
	lip, lport, err1 := parseHexAddr(f[1])
	rip, rport, err2 := parseHexAddr(f[2])
	st, err3 := strconv.ParseUint(f[3], 16, 8)
	ino, err4 := strconv.ParseUint(f[9], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return Socket{}, false
	}
	state := StateOther
	if proto == "tcp" {
		switch st {
		case 0x0A:
			state = StateListen
		case 0x01:
			state = StateEstablished
		}
	} else if st == 0x07 {
		state = StateListen
	}
	return Socket{
		Proto: proto, LocalIP: lip, LocalPort: lport,
		RemoteIP: rip, RemotePort: rport, State: state, Inode: ino,
	}, true
}

// parseHexAddr decodes /proc/net's "0100007F:1F90" address form. IP bytes
// are little-endian within each 4-byte group.
func parseHexAddr(s string) (netip.Addr, uint16, error) {
	ipHex, portHex, ok := strings.Cut(s, ":")
	if !ok {
		return netip.Addr{}, 0, fmt.Errorf("bad address %q", s)
	}
	port, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil {
		return netip.Addr{}, 0, err
	}
	b, err := hex.DecodeString(ipHex)
	if err != nil {
		return netip.Addr{}, 0, err
	}
	for i := 0; i+4 <= len(b); i += 4 {
		b[i], b[i+1], b[i+2], b[i+3] = b[i+3], b[i+2], b[i+1], b[i]
	}
	switch len(b) {
	case 4:
		return netip.AddrFrom4([4]byte(b)), uint16(port), nil
	case 16:
		return netip.AddrFrom16([16]byte(b)).Unmap(), uint16(port), nil
	}
	return netip.Addr{}, 0, fmt.Errorf("bad address length %d", len(b))
}
