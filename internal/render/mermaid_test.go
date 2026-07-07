package render

import "testing"

func TestMermaid(t *testing.T) {
	got := Mermaid(testGraph())
	want := `flowchart LR
    n0["mariadb.service<br/>:3306"]
    n1["myapp.service<br/>:3000"]
    n2["nginx.service<br/>:80, :443"]
    n3["webapp [docker]<br/>127.0.0.1:8080"]
    n4(["10.0.0.5:5432"])
    n1 -.->|connects :3306| n0
    n1 -.->|connects :5432| n4
    n2 -->|proxies :3000| n1
`
	if got != want {
		t.Errorf("Mermaid output mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestMermaidEscape(t *testing.T) {
	cases := []struct{ in, want string }{
		{`plain`, `plain`},
		{`say "hi"`, `say #quot;hi#quot;`},
		{"line\nbreak", "line break"},
		{`back\slash`, `back\\slash`},
	}
	for _, c := range cases {
		if got := mermaidEscape(c.in); got != c.want {
			t.Errorf("mermaidEscape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
