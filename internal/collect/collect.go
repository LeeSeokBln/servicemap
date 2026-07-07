package collect

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Snapshot struct {
	Sockets     []Socket
	Processes   []Process
	Nginx       *NginxConfig      // nil when no nginx or config unreadable
	DockerNames map[string]string // container ID -> name
	SkippedPIDs int               // processes whose fd table was unreadable
	Warnings    []string
}

// Collect gathers everything servicemap needs from the filesystem root
// (normally "/"; a fixture directory in tests).
func Collect(root string) (*Snapshot, error) {
	snap := &Snapshot{DockerNames: map[string]string{}}
	procs, skipped, err := ScanProcesses(root)
	if err != nil {
		return nil, err
	}
	snap.Processes = procs
	snap.SkippedPIDs = skipped

	sockets := map[uint64]Socket{}
	host, err := ReadSockets(filepath.Join(root, "proc", "net"))
	if err != nil {
		return nil, err
	}
	for _, s := range host {
		sockets[s.Inode] = s
	}
	hostNS := ""
	if link, err := os.Readlink(filepath.Join(root, "proc", "1", "ns", "net")); err == nil {
		hostNS = link
	}
	seen := map[string]bool{hostNS: true, "": true}
	for _, p := range procs {
		if seen[p.NetNS] {
			continue
		}
		seen[p.NetNS] = true
		extra, err := ReadSockets(filepath.Join(root, "proc", strconv.Itoa(p.PID), "net"))
		if err != nil {
			continue
		}
		for _, s := range extra {
			sockets[s.Inode] = s
		}
	}
	for _, s := range sockets {
		snap.Sockets = append(snap.Sockets, s)
	}
	sort.Slice(snap.Sockets, func(i, j int) bool { return snap.Sockets[i].Inode < snap.Sockets[j].Inode })

	ids := map[string]bool{}
	for _, p := range procs {
		if p.ContainerID != "" {
			ids[p.ContainerID] = true
		}
	}
	if len(ids) > 0 {
		list := make([]string, 0, len(ids))
		for id := range ids {
			list = append(list, id)
		}
		sort.Strings(list)
		snap.DockerNames = ResolveDockerNames(filepath.Join(root, "var", "run", "docker.sock"), list)
	}

	if path := nginxConfigPath(root, procs); path != "" {
		cfg, err := ParseNginxConfig(root, path)
		if err != nil {
			snap.Warnings = append(snap.Warnings, "nginx config not readable: "+err.Error())
		} else {
			snap.Nginx = cfg
		}
	}
	return snap, nil
}

// nginxConfigPath returns the nginx config path under root, or "" when no
// nginx process is running. nginx rewrites argv, so the -c flag is found by
// searching the joined cmdline string.
func nginxConfigPath(root string, procs []Process) string {
	hasNginx := false
	for _, p := range procs {
		if p.Comm != "nginx" {
			continue
		}
		hasNginx = true
		args := strings.Join(p.Cmdline, " ")
		if i := strings.Index(args, "-c "); i >= 0 {
			rest := strings.Fields(args[i+len("-c "):])
			if len(rest) > 0 {
				return filepath.Join(root, rest[0])
			}
		}
	}
	if !hasNginx {
		return ""
	}
	return filepath.Join(root, "etc", "nginx", "nginx.conf")
}
