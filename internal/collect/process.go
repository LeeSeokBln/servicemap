package collect

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type Process struct {
	PID          int
	Comm         string
	Cmdline      []string
	Unit         string // systemd unit ("nginx.service"), "" if none
	ContainerID  string // docker container ID, "" if none
	NetNS        string // "net:[4026531840]", "" if unreadable
	SocketInodes []uint64
}

var (
	reDockerScope = regexp.MustCompile(`docker-([0-9a-f]{12,64})\.scope`)
	reDockerPath  = regexp.MustCompile(`/docker/([0-9a-f]{12,64})(?:/|$)`)
	reUnit        = regexp.MustCompile(`/([^/]+\.service)(?:/|$)`)
	reUserUnit    = regexp.MustCompile(`^user@\d+\.service$`)
)

// ScanProcesses walks <root>/proc/<pid> and returns processes plus the count
// of processes whose fd table could not be read (permissions).
func ScanProcesses(root string) ([]Process, int, error) {
	procDir := filepath.Join(root, "proc")
	entries, err := os.ReadDir(procDir)
	if err != nil {
		return nil, 0, err
	}
	var procs []Process
	skipped := 0
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || !e.IsDir() {
			continue
		}
		dir := filepath.Join(procDir, e.Name())
		comm, err := os.ReadFile(filepath.Join(dir, "comm"))
		if err != nil {
			continue // process vanished mid-scan
		}
		p := Process{PID: pid, Comm: strings.TrimSpace(string(comm))}
		if cmd, err := os.ReadFile(filepath.Join(dir, "cmdline")); err == nil {
			for _, a := range strings.Split(strings.TrimRight(string(cmd), "\x00"), "\x00") {
				if a != "" {
					p.Cmdline = append(p.Cmdline, a)
				}
			}
		}
		if cg, err := os.ReadFile(filepath.Join(dir, "cgroup")); err == nil {
			p.Unit, p.ContainerID = parseCgroup(string(cg))
		}
		if ns, err := os.Readlink(filepath.Join(dir, "ns", "net")); err == nil {
			p.NetNS = ns
		}
		fdDir := filepath.Join(dir, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			skipped++
		} else {
			for _, fd := range fds {
				link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
				if err != nil {
					continue
				}
				if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
					if ino, err := strconv.ParseUint(link[8:len(link)-1], 10, 64); err == nil {
						p.SocketInodes = append(p.SocketInodes, ino)
					}
				}
			}
			sort.Slice(p.SocketInodes, func(i, j int) bool { return p.SocketInodes[i] < p.SocketInodes[j] })
		}
		procs = append(procs, p)
	}
	sort.Slice(procs, func(i, j int) bool { return procs[i].PID < procs[j].PID })
	return procs, skipped, nil
}

func parseCgroup(data string) (unit, containerID string) {
	for _, line := range strings.Split(data, "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		path := parts[2]
		if m := reDockerScope.FindStringSubmatch(path); m != nil {
			containerID = m[1]
		} else if m := reDockerPath.FindStringSubmatch(path); m != nil {
			containerID = m[1]
		}
		for _, m := range reUnit.FindAllStringSubmatch(path, -1) {
			if !reUserUnit.MatchString(m[1]) {
				unit = m[1]
			}
		}
	}
	if containerID != "" {
		unit = "" // containerized process: label by container, not by docker's unit
	}
	return unit, containerID
}
