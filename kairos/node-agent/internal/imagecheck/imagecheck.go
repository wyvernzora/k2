package imagecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wyvernzora/k2/kairos/node-agent/internal/buildmetadata"
	"github.com/wyvernzora/k2/kairos/node-agent/internal/textfile"
)

const (
	DefaultTextfileDir = "/var/lib/prometheus/node-exporter"

	textfileName       = "k2-image.prom"
	defaultDigestFile  = "/usr/local/.state/k2-image-digest"
	defaultTokenURL    = "https://ghcr.io/token?scope=repository:wyvernzora/k2-kairos:pull"
	defaultRegistryURL = "https://ghcr.io/v2/wyvernzora/k2-kairos"
	sourceCommitLabel  = "org.opencontainers.image.revision"
	requestTimeout     = 10 * time.Second
	overallTimeout     = 30 * time.Second
	maxResponseBytes   = 1 << 20

	// GHCR can return an unusable manifest representation when these media
	// types are omitted, so every manifest request advertises all three.
	manifestAccept = "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json"
)

type Config struct {
	TextfileDir  string
	MetadataFile string
	DigestFile   string
	HTTPClient   *http.Client
	TokenURL     string
	RegistryURL  string
	Log          io.Writer
}

type result struct {
	success          bool
	upgradeAvailable bool
	appliedDigest    string
	registryDigest   string
	tag              string
}

type descriptor struct {
	Digest   string    `json:"digest"`
	Platform *platform `json:"platform,omitempty"`
}

type platform struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
}

type manifestDocument struct {
	Config    descriptor   `json:"config"`
	Manifests []descriptor `json:"manifests"`
}

// Run checks the floating image tag and atomically writes k2-image.prom even
// when the check fails, so a registry outage cannot look like an up-to-date node.
func Run(cfg Config) error {
	cfg = normalize(cfg)
	logf(cfg.Log, "starting registry check")

	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()
	result, checkErr := check(ctx, cfg)
	writeErr := textfile.Write(
		filepath.Join(cfg.TextfileDir, textfileName),
		render(result, time.Now().Unix()),
	)
	if writeErr != nil {
		writeErr = fmt.Errorf("write image check metrics: %w", writeErr)
		if checkErr != nil {
			return errors.Join(checkErr, writeErr)
		}
		return writeErr
	}
	if checkErr != nil {
		logf(cfg.Log, "recorded failed registry check")
		return checkErr
	}

	logf(cfg.Log, "completed registry check for tag %s: upgrade_available=%t", result.tag, result.upgradeAvailable)
	return nil
}

func check(ctx context.Context, cfg Config) (result, error) {
	metadata, err := buildmetadata.Read(cfg.MetadataFile)
	if err != nil {
		return result{}, err
	}
	tag, err := slotTag(metadata)
	if err != nil {
		return result{}, err
	}
	token, err := fetchToken(ctx, cfg)
	if err != nil {
		return result{}, fmt.Errorf("request registry token: %w", err)
	}
	registryDigest, err := fetchDigest(ctx, cfg, token, tag)
	if err != nil {
		return result{}, fmt.Errorf("resolve registry digest for %s: %w", tag, err)
	}

	applied, err := os.ReadFile(cfg.DigestFile)
	if err == nil {
		appliedDigest := strings.TrimSpace(string(applied))
		if appliedDigest == "" {
			return result{}, fmt.Errorf("applied image digest file %s is empty", cfg.DigestFile)
		}
		return result{
			success:          true,
			upgradeAvailable: appliedDigest != registryDigest,
			appliedDigest:    appliedDigest,
			registryDigest:   registryDigest,
			tag:              tag,
		}, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return result{}, fmt.Errorf("read applied image digest: %w", err)
	}

	sourceCommit := strings.TrimSpace(metadata["sourceCommit"])
	if sourceCommit == "" {
		return result{}, errors.New("image build metadata has empty sourceCommit")
	}
	registryCommit, err := fetchRevision(ctx, cfg, token, tag, metadata["arch"])
	if err != nil {
		return result{}, fmt.Errorf("read registry revision for %s: %w", tag, err)
	}
	if registryCommit == "" {
		return result{}, fmt.Errorf("registry image %s has empty %s label", tag, sourceCommitLabel)
	}
	return result{
		success:          true,
		upgradeAvailable: registryCommit != sourceCommit,
		registryDigest:   registryDigest,
		tag:              tag,
	}, nil
}

// Keep this aligned with tagSegments in tools/internal/image/plan/plan.go;
// the separate Go modules cannot share the helper without a new shared module.
func slotTag(metadata map[string]string) (string, error) {
	flavor, _, _ := strings.Cut(metadata["flavor"], "-")
	segments := []string{flavor, metadata["arch"], metadata["hardware"], metadata["role"]}
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			return "", errors.New("image build metadata cannot reconstruct the floating slot tag")
		}
	}
	return strings.Join(segments, "-"), nil
}

func fetchToken(ctx context.Context, cfg Config) (string, error) {
	_, body, err := request(ctx, cfg, http.MethodGet, cfg.TokenURL, "", "application/json")
	if err != nil {
		return "", err
	}
	var response struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if response.Token == "" {
		return "", errors.New("token response is empty")
	}
	return response.Token, nil
}

func fetchDigest(ctx context.Context, cfg Config, token, tag string) (string, error) {
	header, _, err := request(ctx, cfg, http.MethodHead, manifestURL(cfg.RegistryURL, tag), token, manifestAccept)
	if err != nil {
		return "", err
	}
	digest := strings.TrimSpace(header.Get("Docker-Content-Digest"))
	if digest == "" {
		return "", errors.New("manifest response has no Docker-Content-Digest header")
	}
	return digest, nil
}

func fetchRevision(ctx context.Context, cfg Config, token, tag, arch string) (string, error) {
	manifest, err := fetchManifest(ctx, cfg, token, tag)
	if err != nil {
		return "", err
	}
	if manifest.Config.Digest == "" {
		var matches []descriptor
		for _, candidate := range manifest.Manifests {
			if candidate.Platform != nil && candidate.Platform.OS == "linux" && candidate.Platform.Architecture == arch {
				matches = append(matches, candidate)
			}
		}
		if len(matches) != 1 {
			return "", fmt.Errorf("manifest index has %d linux/%s image manifests, want 1", len(matches), arch)
		}
		manifest, err = fetchManifest(ctx, cfg, token, matches[0].Digest)
		if err != nil {
			return "", err
		}
	}
	if manifest.Config.Digest == "" {
		return "", errors.New("image manifest has no config digest")
	}

	_, body, err := request(ctx, cfg, http.MethodGet, blobURL(cfg.RegistryURL, manifest.Config.Digest), token, "application/json")
	if err != nil {
		return "", err
	}
	var config struct {
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"config"`
	}
	if err := json.Unmarshal(body, &config); err != nil {
		return "", fmt.Errorf("decode image config: %w", err)
	}
	return strings.TrimSpace(config.Config.Labels[sourceCommitLabel]), nil
}

func fetchManifest(ctx context.Context, cfg Config, token, reference string) (manifestDocument, error) {
	_, body, err := request(ctx, cfg, http.MethodGet, manifestURL(cfg.RegistryURL, reference), token, manifestAccept)
	if err != nil {
		return manifestDocument{}, err
	}
	var manifest manifestDocument
	if err := json.Unmarshal(body, &manifest); err != nil {
		return manifestDocument{}, fmt.Errorf("decode image manifest: %w", err)
	}
	return manifest, nil
}

func request(ctx context.Context, cfg Config, method, requestURL, token, accept string) (http.Header, []byte, error) {
	requestCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, method, requestURL, http.NoBody)
	if err != nil {
		return nil, nil, fmt.Errorf("create %s request: %w", method, err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%s %s: %w", method, requestURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, nil, fmt.Errorf("%s %s returned %s", method, requestURL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read %s %s response: %w", method, requestURL, err)
	}
	if len(body) > maxResponseBytes {
		return nil, nil, fmt.Errorf("%s %s response exceeds %d bytes", method, requestURL, maxResponseBytes)
	}
	return resp.Header, body, nil
}

func manifestURL(registryURL, reference string) string {
	return strings.TrimRight(registryURL, "/") + "/manifests/" + url.PathEscape(reference)
}

func blobURL(registryURL, digest string) string {
	return strings.TrimRight(registryURL, "/") + "/blobs/" + url.PathEscape(digest)
}

func render(result result, timestamp int64) string {
	var b strings.Builder
	if result.success {
		b.WriteString("# HELP k2_node_image_upgrade_available Whether a newer K2 node image is available.\n")
		b.WriteString("# TYPE k2_node_image_upgrade_available gauge\n")
		fmt.Fprintf(&b, "k2_node_image_upgrade_available %d\n", boolValue(result.upgradeAvailable))
	}
	b.WriteString("# HELP k2_node_image_check_success Whether the K2 node image registry check succeeded.\n")
	b.WriteString("# TYPE k2_node_image_check_success gauge\n")
	fmt.Fprintf(&b, "k2_node_image_check_success %d\n", boolValue(result.success))
	b.WriteString("# HELP k2_node_image_check_timestamp_seconds Unix timestamp of the latest K2 node image registry check.\n")
	b.WriteString("# TYPE k2_node_image_check_timestamp_seconds gauge\n")
	fmt.Fprintf(&b, "k2_node_image_check_timestamp_seconds %d\n", timestamp)
	if result.success {
		b.WriteString("# HELP k2_node_image_info K2 node applied and registry image identity.\n")
		b.WriteString("# TYPE k2_node_image_info gauge\n")
		fmt.Fprintf(&b, "k2_node_image_info{applied_digest=%q,registry_digest=%q,tag=%q} 1\n", result.appliedDigest, result.registryDigest, result.tag)
	}
	return b.String()
}

func boolValue(value bool) int {
	if value {
		return 1
	}
	return 0
}

func normalize(cfg Config) Config {
	if cfg.TextfileDir == "" {
		cfg.TextfileDir = DefaultTextfileDir
	}
	if cfg.MetadataFile == "" {
		cfg.MetadataFile = buildmetadata.DefaultPath
	}
	if cfg.DigestFile == "" {
		cfg.DigestFile = defaultDigestFile
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: requestTimeout}
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = defaultTokenURL
	}
	if cfg.RegistryURL == "" {
		cfg.RegistryURL = defaultRegistryURL
	}
	if cfg.Log == nil {
		cfg.Log = io.Discard
	}
	return cfg
}

func logf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, "k2-node-agent image-check: "+format+"\n", args...)
}
