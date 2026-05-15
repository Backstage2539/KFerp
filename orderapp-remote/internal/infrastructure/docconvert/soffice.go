package docconvert

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type CommandRunner func(ctx context.Context, name string, args ...string) error

type LibreOfficeConverter struct {
	command string
	runner  CommandRunner
}

type Option func(*LibreOfficeConverter)

func WithCommandRunner(runner CommandRunner) Option {
	return func(c *LibreOfficeConverter) {
		if runner != nil {
			c.runner = runner
		}
	}
}

func NewLibreOfficeConverter(command string, opts ...Option) LibreOfficeConverter {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "soffice"
	}
	c := LibreOfficeConverter{command: command, runner: runCommand}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

func (c LibreOfficeConverter) ConvertDOCXToPDF(ctx context.Context, sourcePath, outputDir string) (string, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	outputDir = strings.TrimSpace(outputDir)
	if sourcePath == "" {
		return "", fmt.Errorf("source path required")
	}
	if outputDir == "" {
		return "", fmt.Errorf("output dir required")
	}
	args := []string{"--headless", "--convert-to", "pdf", "--outdir", outputDir, sourcePath}
	if err := c.runner(ctx, c.command, args...); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath)) + ".pdf"
	return filepath.Join(outputDir, base), nil
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s failed: %s", name, msg)
	}
	return nil
}
