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

var benchmarkIgnorePaths = []string{
	"node_modules",
	"dist",
	"coverage",
	"packages/*/dist",
}

const benchmarkPackageVariants = 25

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
		dir := fmt.Sprintf("docs/group-%03d", i%benchmarkPackageVariants)
		files[dir] = &fstest.MapFile{Mode: fs.ModeDir}
		files[fmt.Sprintf("%s/guide-%05d.md", dir, i)] = &fstest.MapFile{Mode: fs.ModePerm, Data: content}
	}

	addIgnoredBenchmarkFiles(files, fileCount, []byte("ignored benchmark artifact\n"), ".md")

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
		dir := fmt.Sprintf("src/module-%03d", i%benchmarkPackageVariants)
		files[dir] = &fstest.MapFile{Mode: fs.ModeDir}
		files[fmt.Sprintf("%s/serviceResult%05d.ts", dir, i)] = &fstest.MapFile{Mode: fs.ModePerm, Data: content}
	}

	addIgnoredBenchmarkFiles(files, fileCount, content, ".ts")

	return files
}

func benchmarkContentConfig(ruleSpec string) *config.Config {
	return config.NewConfig(config.Ls{
		"docs": config.Ls{
			"**": config.Ls{
				".md": ruleSpec,
			},
		},
	}, benchmarkIgnorePaths)
}

func benchmarkCodeConfig(ruleSpec string) *config.Config {
	return config.NewConfig(config.Ls{
		"src": config.Ls{
			"**": config.Ls{
				".ts": ruleSpec,
			},
		},
	}, benchmarkIgnorePaths)
}

func addIgnoredBenchmarkFiles(files fstest.MapFS, fileCount int, content []byte, extension string) {
	ignoredFileCount := fileCount / 5
	if ignoredFileCount < 10 {
		ignoredFileCount = 10
	}

	files["node_modules"] = &fstest.MapFile{Mode: fs.ModeDir}
	files["dist"] = &fstest.MapFile{Mode: fs.ModeDir}
	files["coverage"] = &fstest.MapFile{Mode: fs.ModeDir}
	files["packages"] = &fstest.MapFile{Mode: fs.ModeDir}

	for i := 0; i < ignoredFileCount; i++ {
		packageName := fmt.Sprintf("pkg-%03d", i%benchmarkPackageVariants)
		nodeModulesDir := fmt.Sprintf("node_modules/%s", packageName)
		distDir := fmt.Sprintf("dist/chunk-%03d", i%benchmarkPackageVariants)
		coverageDir := fmt.Sprintf("coverage/run-%03d", i%benchmarkPackageVariants)
		packageDir := fmt.Sprintf("packages/%s", packageName)
		packageDistDir := fmt.Sprintf("%s/dist", packageDir)

		files[nodeModulesDir] = &fstest.MapFile{Mode: fs.ModeDir}
		files[distDir] = &fstest.MapFile{Mode: fs.ModeDir}
		files[coverageDir] = &fstest.MapFile{Mode: fs.ModeDir}
		files[packageDir] = &fstest.MapFile{Mode: fs.ModeDir}
		files[packageDistDir] = &fstest.MapFile{Mode: fs.ModeDir}

		files[fmt.Sprintf("%s/ignored%05d%s", nodeModulesDir, i, extension)] = &fstest.MapFile{Mode: fs.ModePerm, Data: content}
		files[fmt.Sprintf("%s/ignored%05d%s", distDir, i, extension)] = &fstest.MapFile{Mode: fs.ModePerm, Data: content}
		files[fmt.Sprintf("%s/ignored%05d.txt", coverageDir, i)] = &fstest.MapFile{Mode: fs.ModePerm, Data: []byte("mode: set\n")}
		files[fmt.Sprintf("%s/ignored%05d%s", packageDistDir, i, extension)] = &fstest.MapFile{Mode: fs.ModePerm, Data: content}
	}
}
