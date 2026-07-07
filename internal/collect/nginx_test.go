package collect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseNginxConfig(t *testing.T) {
	root := t.TempDir()
	confDir := filepath.Join(root, "etc", "nginx")
	os.MkdirAll(filepath.Join(confDir, "conf.d"), 0o755)

	main := `# main config
user www-data;
events { worker_connections 768; }
http {
    upstream app {
        server 127.0.0.1:3000 weight=2;
        server 127.0.0.1:3001;
    }
    include conf.d/*.conf;
}
`
	site := `server {
    listen 80;
    listen [::]:80;
    location / {
        proxy_pass http://app;
    }
    location /api/ {
        proxy_pass http://127.0.0.1:9090/api/;
    }
    location ~ \.php$ {
        fastcgi_pass 127.0.0.1:9000;
    }
}
`
	os.WriteFile(filepath.Join(confDir, "nginx.conf"), []byte(main), 0o644)
	os.WriteFile(filepath.Join(confDir, "conf.d", "site.conf"), []byte(site), 0o644)

	cfg, err := ParseNginxConfig(root, filepath.Join(confDir, "nginx.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Upstreams["app"]; len(got) != 2 || got[0] != "127.0.0.1:3000" || got[1] != "127.0.0.1:3001" {
		t.Errorf("upstream app = %v", got)
	}
	if len(cfg.ProxyPass) != 2 || cfg.ProxyPass[0] != "http://app" || cfg.ProxyPass[1] != "http://127.0.0.1:9090/api/" {
		t.Errorf("ProxyPass = %v", cfg.ProxyPass)
	}
	if len(cfg.FastCGIPass) != 1 || cfg.FastCGIPass[0] != "127.0.0.1:9000" {
		t.Errorf("FastCGIPass = %v", cfg.FastCGIPass)
	}
	if len(cfg.Listens) != 2 || cfg.Listens[0] != "80" || cfg.Listens[1] != "[::]:80" {
		t.Errorf("Listens = %v", cfg.Listens)
	}
}

func TestParseNginxConfigAbsoluteInclude(t *testing.T) {
	root := t.TempDir()
	confDir := filepath.Join(root, "etc", "nginx")
	os.MkdirAll(filepath.Join(confDir, "conf.d"), 0o755)
	os.WriteFile(filepath.Join(confDir, "nginx.conf"),
		[]byte("http { include /etc/nginx/conf.d/*.conf; }\n"), 0o644)
	os.WriteFile(filepath.Join(confDir, "conf.d", "a.conf"),
		[]byte("server { proxy_pass http://127.0.0.1:5000; }\n"), 0o644)

	cfg, err := ParseNginxConfig(root, filepath.Join(confDir, "nginx.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ProxyPass) != 1 || cfg.ProxyPass[0] != "http://127.0.0.1:5000" {
		t.Errorf("ProxyPass = %v", cfg.ProxyPass)
	}
}

func TestParseNginxConfigMissing(t *testing.T) {
	if _, err := ParseNginxConfig(t.TempDir(), "/nonexistent/nginx.conf"); err == nil {
		t.Error("want error for unreadable root config")
	}
}

func TestParseNginxConfigIncludeCycle(t *testing.T) {
	root := t.TempDir()
	confDir := filepath.Join(root, "etc", "nginx")
	os.MkdirAll(confDir, 0o755)
	os.WriteFile(filepath.Join(confDir, "nginx.conf"),
		[]byte("include nginx.conf;\nproxy_pass http://127.0.0.1:1;\n"), 0o644)
	cfg, err := ParseNginxConfig(root, filepath.Join(confDir, "nginx.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ProxyPass) != 1 {
		t.Errorf("cycle not guarded: ProxyPass = %v", cfg.ProxyPass)
	}
}
