package appmain

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type DocsIndexData struct {
	Files []string
	Error string
}

type DocViewData struct {
	Name    string
	Content string
	Error   string
}

func listDocFiles(dir string) ([]string, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0)
	for _, e := range es {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func safeDocName(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = filepath.Base(name)
	if name == "." || name == "" {
		return "", fmt.Errorf("invalid name")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		return "", fmt.Errorf("only .md allowed")
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid path")
	}
	return name, nil
}

// template helper: render markdown-ish content as plain text inside <pre>
func pre(s string) template.HTML {
	// We deliberately escape by letting template engine escape in <pre>.
	// This helper exists mainly for future extension; currently unused.
	_ = s
	return ""
}
