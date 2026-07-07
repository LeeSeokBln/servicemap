package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LeeSeokBln/servicemap/internal/collect"
	"github.com/LeeSeokBln/servicemap/internal/graph"
	"github.com/LeeSeokBln/servicemap/internal/render"
	"github.com/spf13/cobra"
)

var version = "dev"

func newRootCmd() *cobra.Command {
	var (
		format   string
		output   string
		all      bool
		procRoot string
	)
	cmd := &cobra.Command{
		Use:           "servicemap",
		Short:         "Map running services, ports, and their relationships",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			real := procRoot == "/"
			if real && runtime.GOOS != "linux" {
				return fmt.Errorf("servicemap requires Linux (/proc); GOOS=%s", runtime.GOOS)
			}
			f, err := resolveFormat(format, output)
			if err != nil {
				return err
			}
			snap, err := collect.Collect(procRoot)
			if err != nil {
				return err
			}
			if real && os.Geteuid() != 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: not running as root — map may be incomplete (%d processes not inspected); try: sudo servicemap\n",
					snap.SkippedPIDs)
			}
			g := graph.Build(snap, graph.Options{All: all})
			for _, w := range g.Warnings {
				fmt.Fprintln(cmd.ErrOrStderr(), "note: "+w)
			}
			out, err := render.Render(g, f)
			if err != nil {
				return err
			}
			if output == "" {
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}
			return os.WriteFile(output, []byte(out), 0o644)
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "",
		"output format: tree, mermaid, md, json (default tree; inferred from --output extension)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "write to file instead of stdout")
	cmd.Flags().BoolVar(&all, "all", false, "show all processes (disable noise filter)")
	cmd.Flags().StringVar(&procRoot, "proc-root", "/", "filesystem root to inspect (for testing)")
	_ = cmd.Flags().MarkHidden("proc-root")
	return cmd
}

func resolveFormat(flag, output string) (string, error) {
	if flag != "" {
		switch flag {
		case "tree", "mermaid", "md", "json":
			return flag, nil
		}
		return "", fmt.Errorf("unknown format %q (valid: tree, mermaid, md, json)", flag)
	}
	switch strings.ToLower(filepath.Ext(output)) {
	case ".mmd", ".mermaid":
		return "mermaid", nil
	case ".md", ".markdown":
		return "md", nil
	case ".json":
		return "json", nil
	}
	return "tree", nil
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error: "+err.Error())
		os.Exit(1)
	}
}
