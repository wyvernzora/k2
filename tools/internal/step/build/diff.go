package build

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wyvernzora/k2/tools/internal/build/dyffignore"
	"github.com/wyvernzora/k2/tools/internal/build/dyffx"
)

func DiffManifests(ctx context.Context, opts DiffManifestsOptions) error {
	repoRoot := opts.RepoRoot
	remoteURL := opts.RemoteURL
	deployRoot := filepath.Join(repoRoot, "deploy")
	if info, err := os.Stat(deployRoot); err != nil {
		return fmt.Errorf("local build artifacts not found; run earthly +k8s-manifests first: %w", err)
	} else if !info.IsDir() {
		return fmt.Errorf("deploy path is not a directory: %s", deployRoot)
	}
	rules, err := dyffignore.Load(filepath.Join(repoRoot, dyffIgnorePath))
	if err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp("", "k2-deploy-diff-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	remoteRoot := filepath.Join(tempDir, "remote")
	hasDeployBranch, err := remoteHasDeployBranch(ctx, repoRoot, remoteURL)
	if err != nil {
		return err
	}
	if hasDeployBranch {
		if _, err := runCapture(ctx, repoRoot, "git", "clone", "--branch", deployBranch, "--depth", "1", remoteURL, remoteRoot); err != nil {
			return err
		}
		if err := os.RemoveAll(filepath.Join(remoteRoot, ".git")); err != nil {
			return err
		}
	} else if err := os.MkdirAll(remoteRoot, 0o755); err != nil {
		return err
	}
	diffEntries, err := gitNameStatus(ctx, repoRoot, remoteRoot, deployRoot)
	if err != nil {
		return err
	}
	var out bytes.Buffer
	for _, entry := range diffEntries {
		if err := writeDiffEntry(&out, entry, remoteRoot, deployRoot, rules); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(repoRoot, "deploy-diff.md"), out.Bytes(), 0o644)
}

func remoteHasDeployBranch(ctx context.Context, dir string, remoteURL string) (bool, error) {
	result := runCommand(ctx, dir, nil, "git", "ls-remote", "--exit-code", "--heads", remoteURL, deployBranch)
	switch result.ExitCode {
	case 0:
		return true, nil
	case 2:
		return false, nil
	default:
		return false, result.Error("git ls-remote")
	}
}

type diffEntry struct {
	Status string
	Path1  string
	Path2  string
}

func gitNameStatus(ctx context.Context, dir string, remoteRoot string, deployRoot string) ([]diffEntry, error) {
	result := runCommand(ctx, dir, nil, "git", "diff", "--no-index", "--name-status", "-z", "-M", "--color=never", remoteRoot, deployRoot)
	if result.Err != nil && result.ExitCode != 1 {
		return nil, result.Error("git diff --no-index")
	}
	return parseGitNameStatus(result.Stdout, remoteRoot, deployRoot)
}

func parseGitNameStatus(data []byte, remoteRoot string, deployRoot string) ([]diffEntry, error) {
	fields := bytes.Split(bytes.TrimRight(data, "\x00"), []byte{0})
	if len(fields) == 1 && len(fields[0]) == 0 {
		return nil, nil
	}
	var entries []diffEntry
	for i := 0; i < len(fields); {
		status := string(fields[i])
		i++
		if i >= len(fields) {
			return nil, fmt.Errorf("parse git name-status: missing path for status %q", status)
		}
		entry := diffEntry{
			Status: status,
			Path1:  cleanupDiffPath(remoteRoot, deployRoot, string(fields[i])),
		}
		i++
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			if i >= len(fields) {
				return nil, fmt.Errorf("parse git name-status: missing destination path for status %q", status)
			}
			entry.Path2 = cleanupDiffPath(remoteRoot, deployRoot, string(fields[i]))
			i++
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func writeDiffEntry(out *bytes.Buffer, entry diffEntry, remoteRoot string, deployRoot string, rules dyffignore.Rules) error {
	appExcludes := appDiffExcludes(rules, entry)
	switch {
	case entry.Status == "A":
		data, err := os.ReadFile(filepath.Join(deployRoot, entry.Path1))
		if err != nil {
			return err
		}
		writeMarkdownDiff(out, "### Added `"+entry.Path1+"`", string(data))
	case entry.Status == "D":
		data, err := os.ReadFile(filepath.Join(remoteRoot, entry.Path1))
		if err != nil {
			return err
		}
		writeMarkdownDiff(out, "### Deleted `"+entry.Path1+"`", string(data))
	case entry.Status == "M":
		diff, different, err := dyffx.BetweenFiles(filepath.Join(remoteRoot, entry.Path1), filepath.Join(deployRoot, entry.Path1), appExcludes)
		if err != nil {
			return err
		}
		if different {
			writeMarkdownDiff(out, "### Modified `"+entry.Path1+"`", diff)
		}
	case strings.HasPrefix(entry.Status, "R"):
		diff, different, err := dyffx.BetweenFiles(filepath.Join(remoteRoot, entry.Path1), filepath.Join(deployRoot, entry.Path2), appExcludes)
		if err != nil {
			return err
		}
		if !different {
			diff = "no changes"
		}
		writeMarkdownDiff(out, "### Renamed `"+entry.Path1+"` -> `"+entry.Path2+"`", diff)
	default:
		return fmt.Errorf("unsupported git diff status %q for %s", entry.Status, entry.Path1)
	}
	return nil
}

func appDiffExcludes(rules dyffignore.Rules, entry diffEntry) []string {
	excludes := rules.Section("app")
	if appName := diffEntryAppName(entry); appName != "" {
		excludes = append(excludes, rules.Section("app:"+appName)...)
	}
	return excludes
}

func diffEntryAppName(entry diffEntry) string {
	path := entry.Path1
	if path == "" {
		path = entry.Path2
	}
	if i := strings.IndexByte(path, '/'); i >= 0 {
		return strings.ToLower(path[:i])
	}
	return ""
}

func writeMarkdownDiff(out *bytes.Buffer, heading string, diff string) {
	diff = strings.TrimLeft(diff, "\n")
	fmt.Fprintf(out, "%s\n```yaml\n%s\n```\n\n", heading, strings.TrimRight(diff, "\n"))
}

func cleanupDiffPath(remoteRoot string, deployRoot string, path string) string {
	if rel, err := filepath.Rel(remoteRoot, path); err == nil && !isOutsideRelativePath(rel) {
		return filepath.ToSlash(rel)
	}
	if rel, err := filepath.Rel(deployRoot, path); err == nil && !isOutsideRelativePath(rel) {
		return filepath.ToSlash(rel)
	}
	if strings.HasPrefix(path, "deploy"+string(filepath.Separator)) {
		return filepath.ToSlash(strings.TrimPrefix(path, "deploy"+string(filepath.Separator)))
	}
	return filepath.ToSlash(path)
}

func isOutsideRelativePath(path string) bool {
	return path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator))
}
