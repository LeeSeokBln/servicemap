package collect

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type NginxConfig struct {
	Upstreams   map[string][]string // upstream name -> server addresses
	ProxyPass   []string            // raw proxy_pass targets
	FastCGIPass []string            // raw fastcgi_pass targets
	Listens     []string            // raw listen directive first-args
}

// ParseNginxConfig parses the nginx config at path (which must live under
// root). Absolute include patterns are re-rooted under root.
func ParseNginxConfig(root, path string) (*NginxConfig, error) {
	cfg := &NginxConfig{Upstreams: map[string][]string{}}
	if err := parseNginxFile(root, path, filepath.Dir(path), cfg, map[string]bool{}, 0); err != nil {
		return nil, err
	}
	return cfg, nil
}

func parseNginxFile(root, path, confDir string, cfg *NginxConfig, visited map[string]bool, depth int) error {
	if depth > 10 || visited[path] {
		return nil
	}
	visited[path] = true
	data, err := os.ReadFile(path)
	if err != nil {
		if depth == 0 {
			return err
		}
		return nil // missing includes are not fatal
	}
	var stack, dir []string
	for _, tok := range tokenizeNginx(string(data)) {
		switch tok {
		case "{":
			name := "block"
			if len(dir) >= 2 && dir[0] == "upstream" {
				name = "upstream:" + dir[1]
			}
			stack = append(stack, name)
			dir = nil
		case "}":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			dir = nil
		case ";":
			handleNginxDirective(root, dir, stack, confDir, cfg, visited, depth)
			dir = nil
		default:
			dir = append(dir, tok)
		}
	}
	return nil
}

func handleNginxDirective(root string, dir, stack []string, confDir string, cfg *NginxConfig, visited map[string]bool, depth int) {
	if len(dir) < 2 {
		return
	}
	switch dir[0] {
	case "include":
		pat := dir[1]
		if filepath.IsAbs(pat) {
			pat = filepath.Join(root, pat)
		} else {
			pat = filepath.Join(confDir, pat)
		}
		matches, _ := filepath.Glob(pat)
		sort.Strings(matches)
		for _, m := range matches {
			_ = parseNginxFile(root, m, confDir, cfg, visited, depth+1)
		}
	case "server":
		if len(stack) > 0 && strings.HasPrefix(stack[len(stack)-1], "upstream:") {
			name := strings.TrimPrefix(stack[len(stack)-1], "upstream:")
			cfg.Upstreams[name] = append(cfg.Upstreams[name], dir[1])
		}
	case "proxy_pass":
		cfg.ProxyPass = append(cfg.ProxyPass, dir[1])
	case "fastcgi_pass":
		cfg.FastCGIPass = append(cfg.FastCGIPass, dir[1])
	case "listen":
		cfg.Listens = append(cfg.Listens, dir[1])
	}
}

// tokenizeNginx splits config text into tokens, treating ";", "{", and "}"
// as separate tokens. Known limitations of the tolerant design: quoted
// strings are not recognized (a "#" inside quotes truncates the line, and
// quoted values containing spaces or ";" mis-tokenize). This trades strict
// correctness for never failing the run on unusual configs.
func tokenizeNginx(src string) []string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if j := strings.IndexByte(line, '#'); j >= 0 {
			lines[i] = line[:j]
		}
	}
	r := strings.NewReplacer(";", " ; ", "{", " { ", "}", " } ")
	return strings.Fields(r.Replace(strings.Join(lines, "\n")))
}
