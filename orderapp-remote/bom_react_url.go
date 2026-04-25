package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const bomReactPath = "/bom-react"

var bomReactDistDir = filepath.Join("frontend", "dist")

func bomReactURL() string {
	return bomReactPath + "?rev=" + currentBomReactRev()
}

func currentBomReactRev() string {
	patterns := []string{
		filepath.Join(bomReactDistDir, "index.html"),
		filepath.Join(bomReactDistDir, "assets", "*.js"),
		filepath.Join(bomReactDistDir, "assets", "*.css"),
	}

	names := make([]string, 0, 4)
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			names = append(names, filepath.Base(match))
		}
	}
	if len(names) == 0 {
		return "dev"
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte('|')
	}
	sum := fnv32a(b.String())
	return fmt.Sprintf("%08x", sum)
}

func fnv32a(s string) uint32 {
	var hash uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= 16777619
	}
	return hash
}

func bomReactIndexExists() bool {
	info, err := os.Stat(filepath.Join(bomReactDistDir, "index.html"))
	return err == nil && !info.IsDir()
}
