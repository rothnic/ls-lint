package linter

import (
	"fmt"
	"io/fs"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/loeffel-io/ls-lint/v2/internal/config"
	"github.com/loeffel-io/ls-lint/v2/internal/debug"
	"github.com/loeffel-io/ls-lint/v2/internal/rule"
)

func BenchmarkLinterRunContentScenarios(b *testing.B) {
	configurations := []struct {
		name string
		spec string
	}{
		{name: "NoContent", spec: "kebab-case"},
		{name: "SingleContent", spec: "kebab-case | content:max-lines:40"},
		{name: "FullContent", spec: "kebab-case | content:max-lines:40 | content:max-line-length:120 | content:heading:^## Overview$ | content:front-matter:required"},
	}

	fileCounts := []struct {
		name  string
		count int
	}{
		{name: "FewFiles", count: 10},
		{name: "ModerateFiles", count: 500},
		{name: "ManyFiles", count: 5000},
	}

	for _, fileCount := range fileCounts {
		filesystem := benchmarkContentFilesystem(fileCount.count)

		for _, cfg := range configurations {
			b.Run(fmt.Sprintf("%s/%s", fileCount.name, cfg.name), func(b *testing.B) {
				lintConfig := benchmarkContentConfig(cfg.spec)
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					l := NewLinter(
						".",
						lintConfig,
						&debug.Statistic{Start: time.Now(), RWMutex: new(sync.RWMutex)},
						[]*rule.Error{},
					)
					if err := l.Run(filesystem, nil, false); err != nil {
						b.Fatalf("unexpected error: %v", err)
					}
				}
			})
		}
	}
}

func BenchmarkLinterRunCodeScenarios(b *testing.B) {
	configurations := []struct {
		name string
		spec string
	}{
		{name: "NamingOnly", spec: "camelCase | PascalCase"},
		{name: "NamingPlusMaxLines", spec: "camelCase | PascalCase | content:max-lines:400"},
		{name: "NamingPlusMaxLinesAndLineLength", spec: "camelCase | PascalCase | content:max-lines:400 | content:max-line-length:120"},
	}

	fileCounts := []struct {
		name  string
		count int
	}{
		{name: "FewFiles", count: 10},
		{name: "ModerateFiles", count: 500},
		{name: "ManyFiles", count: 5000},
	}

	for _, fileCount := range fileCounts {
		filesystem := benchmarkCodeFilesystem(fileCount.count)

		for _, cfg := range configurations {
			b.Run(fmt.Sprintf("%s/%s", fileCount.name, cfg.name), func(b *testing.B) {
				lintConfig := benchmarkCodeConfig(cfg.spec)
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					l := NewLinter(
						".",
						lintConfig,
						&debug.Statistic{Start: time.Now(), RWMutex: new(sync.RWMutex)},
						[]*rule.Error{},
					)
					if err := l.Run(filesystem, nil, false); err != nil {
						b.Fatalf("unexpected error: %v", err)
					}
				}
			})
		}
	}
}

func benchmarkContentFilesystem(fileCount int) fs.FS {
	files := fstest.MapFS{
		"docs": &fstest.MapFile{Mode: fs.ModeDir},
	}

	content := []byte("---\ntitle: Guide\nsummary: Example\n---\n# Intro\n## Overview\nThis is a realistic markdown file used for performance measurements.\nIt has enough lines to exercise max-lines and max-line-length checks.\nThe file stays comfortably under the configured thresholds.\n## Usage\nUse the content directive to enforce lightweight structure.\nAnother line of text to keep the file shape realistic.\nFinal line.\n")

	for i := 0; i < fileCount; i++ {
		dir := fmt.Sprintf("docs/group-%03d", i%25)
		files[dir] = &fstest.MapFile{Mode: fs.ModeDir}
		files[fmt.Sprintf("%s/guide-%05d.md", dir, i)] = &fstest.MapFile{Mode: fs.ModePerm, Data: content}
	}

	return files
}

func benchmarkCodeFilesystem(fileCount int) fs.FS {
	files := fstest.MapFS{
		"src": &fstest.MapFile{Mode: fs.ModeDir},
	}

	content := []byte(`export type ServiceInput = {
  readonly requestId: string;
  readonly includeArchived: boolean;
};

export type ServiceResult = {
  readonly requestId: string;
  readonly itemCount: number;
  readonly summary: string;
};

const summarize = (itemCount: number): string => {
  if (itemCount === 0) {
    return "No matching items were found for the current request.";
  }

  if (itemCount === 1) {
    return "Found a single matching item for the current request.";
  }

  return "Found multiple matching items for the current request.";
};

export const buildServiceResult = (
  input: ServiceInput,
  itemCount: number,
): ServiceResult => {
  return {
    requestId: input.requestId,
    itemCount,
    summary: summarize(itemCount),
  };
};

export const shouldRefreshCache = (
  currentVersion: number,
  nextVersion: number,
): boolean => {
  if (nextVersion <= currentVersion) {
    return false;
  }

  return nextVersion - currentVersion > 2;
};
`)

	for i := 0; i < fileCount; i++ {
		dir := fmt.Sprintf("src/module-%03d", i%25)
		files[dir] = &fstest.MapFile{Mode: fs.ModeDir}
		files[fmt.Sprintf("%s/serviceResult%05d.ts", dir, i)] = &fstest.MapFile{Mode: fs.ModePerm, Data: content}
	}

	return files
}

func benchmarkContentConfig(ruleSpec string) *config.Config {
	return config.NewConfig(config.Ls{
		"docs": config.Ls{
			"**": config.Ls{
				".md": ruleSpec,
			},
		},
	}, nil)
}

func benchmarkCodeConfig(ruleSpec string) *config.Config {
	return config.NewConfig(config.Ls{
		"src": config.Ls{
			"**": config.Ls{
				".ts": ruleSpec,
			},
		},
	}, nil)
}
