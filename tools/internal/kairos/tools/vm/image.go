package vm

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func convertRawXZ(rawXZ string, rawTmp string, qcow2 string) error {
	if _, err := os.Stat(qcow2); err == nil {
		return fmt.Errorf("qcow2 already exists: %s", qcow2)
	}
	rawFile, err := os.Create(rawTmp)
	if err != nil {
		return err
	}
	var xzErr bytes.Buffer
	cmd := exec.Command("xz", "-dc", rawXZ)
	cmd.Stdout = rawFile
	cmd.Stderr = &xzErr // raw tty writes corrupt the ui progress renderer
	err = cmd.Run()
	if err != nil && xzErr.Len() > 0 {
		err = fmt.Errorf("%w\n%s", err, strings.TrimSpace(xzErr.String()))
	}
	closeErr := rawFile.Close()
	if err != nil {
		_ = os.Remove(rawTmp)
		return err
	}
	if closeErr != nil {
		_ = os.Remove(rawTmp)
		return closeErr
	}
	defer os.Remove(rawTmp)
	return runCommand(exec.Command("qemu-img", "convert", "-f", "raw", "-O", "qcow2", rawTmp, qcow2))
}

func resolveArtifact(repoRoot string, override string, target string) (string, error) {
	rawXZ := override
	var err error
	if rawXZ == "" {
		rawXZ, err = artifactForTarget(repoRoot, target)
		if err != nil {
			return "", err
		}
	}
	rawXZ, err = filepath.Abs(rawXZ)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(rawXZ); err != nil {
		return "", fmt.Errorf("raw artifact does not exist: %s: %w", rawXZ, err)
	}
	return rawXZ, nil
}

func artifactForTarget(repoRoot string, target string) (string, error) {
	if artifact, err := localArtifactForTarget(repoRoot, target); err == nil {
		return artifact, nil
	}
	if artifact, err := cachedOrDownloadedArtifactForTarget(repoRoot, target); err == nil {
		return artifact, nil
	}
	return "", fmt.Errorf("no local or cached .raw.xz artifact found for %s, and latest artifact download failed\nBuild it locally with:\n  earthly --allow-privileged ./kairos+image-build-artifact --KAIROS_TARGET=%s", target, target)
}

func localArtifactForTarget(repoRoot string, target string) (string, error) {
	dir := filepath.Join(repoRoot, "kairos", "artifacts", target)
	if manifest, err := readArtifactManifest(filepath.Join(dir, "artifact-manifest.json")); err == nil {
		compressed := manifest.localCompressed()
		if compressed.File != "" {
			path := filepath.Join(dir, compressed.File)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}
	matches, err := filepath.Glob(filepath.Join(dir, "*.raw.xz"))
	if err != nil {
		return "", err
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return "", os.ErrNotExist
	}
	return matches[0], nil
}

func cachedOrDownloadedArtifactForTarget(repoRoot string, target string) (string, error) {
	prefix := strings.Trim(os.Getenv("K2_KAIROS_IMAGE_PREFIX"), "/")
	latestURL := artifactURL(s3Key(prefix, "latest/"+target+"/manifest.json"))
	cacheRoot := os.Getenv("K2_TOOLS_ARTIFACT_CACHE")
	if cacheRoot == "" {
		cacheRoot = filepath.Join(repoRoot, ".testvm", "cache", "artifacts")
	}
	targetCache := filepath.Join(cacheRoot, target)
	latestCache := filepath.Join(targetCache, "latest-manifest.json")
	if err := os.MkdirAll(targetCache, 0o755); err != nil {
		return "", err
	}
	if data, err := download(latestURL); err == nil {
		if err := os.WriteFile(latestCache, data, 0o644); err != nil {
			return "", err
		}
	} else if _, statErr := os.Stat(latestCache); statErr != nil {
		return "", err
	}

	manifest, err := readArtifactManifest(latestCache)
	if err != nil {
		return "", err
	}
	compressed := manifest.remoteCompressed()
	if manifest.Git.SHA == "" || manifest.S3.Prefix == "" || compressed.File == "" || compressed.SHA256 == "" {
		return "", fmt.Errorf("artifact manifest for %s is incomplete", target)
	}
	artifactDir := filepath.Join(targetCache, manifest.Git.SHA)
	artifactPath := filepath.Join(artifactDir, compressed.File)
	if verifySHA256(artifactPath, compressed.SHA256) == nil {
		return artifactPath, nil
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return "", err
	}
	data, err := download(artifactURL(s3Key(manifest.S3.Prefix, compressed.File)))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(artifactPath, data, 0o644); err != nil {
		return "", err
	}
	return artifactPath, verifySHA256(artifactPath, compressed.SHA256)
}

func readArtifactManifest(path string) (artifactManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return artifactManifest{}, err
	}
	var manifest artifactManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return artifactManifest{}, err
	}
	return manifest, nil
}

func (m artifactManifest) localCompressed() artifactFile {
	if m.Artifact.Compressed.File != "" {
		return m.Artifact.Compressed
	}
	return m.Compressed
}

func (m artifactManifest) remoteCompressed() artifactFile {
	if m.Compressed.File != "" {
		return m.Compressed
	}
	return m.Artifact.Compressed
}

func s3Key(prefix string, suffix string) string {
	suffix = strings.TrimLeft(suffix, "/")
	if prefix == "" {
		return suffix
	}
	return strings.TrimRight(prefix, "/") + "/" + suffix
}

func artifactURL(key string) string {
	baseURL := strings.TrimRight(os.Getenv("K2_KAIROS_IMAGE_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = defaultArtifactBaseURL
	}
	return baseURL + "/" + strings.TrimLeft(key, "/")
}

func download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	var reader io.Reader = resp.Body
	if strings.HasSuffix(url, ".gz") || resp.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}
	return io.ReadAll(reader)
}

func verifySHA256(path string, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", path, actual, expected)
	}
	return nil
}
