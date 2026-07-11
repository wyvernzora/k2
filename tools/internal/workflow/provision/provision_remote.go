package provision

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wyvernzora/k2/tools/internal/client/kubectl"
	"github.com/wyvernzora/k2/tools/internal/client/remote"
	"github.com/wyvernzora/k2/tools/internal/clusterconfig"
	"github.com/wyvernzora/k2/tools/internal/kubeconfig"
	"github.com/wyvernzora/k2/tools/internal/render"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

func logf(format string, args ...any) {
	currentReporter().Infof(format, args...)
}

func successf(format string, args ...any) {
	currentReporter().Successf(format, args...)
}

func warnf(format string, args ...any) {
	currentReporter().Warnf(format, args...)
}

func readRemoteMetadata(client *remote.Client) (render.ImageMetadata, error) {
	data, err := client.ReadFile("/usr/share/k2/image-build/metadata.yaml")
	if err != nil {
		return render.ImageMetadata{}, fmt.Errorf("read remote image metadata: %w", err)
	}
	metadata, err := render.DecodeImageMetadata(data)
	if err != nil {
		return render.ImageMetadata{}, err
	}
	if metadata.Target == "" || metadata.Arch == "" || metadata.Hardware == "" {
		return render.ImageMetadata{}, fmt.Errorf("remote image metadata is incomplete; target, arch, and hardware are required")
	}
	return metadata, nil
}

func markLonghornStorageWorker(ctx context.Context, clusterName string, nodeName string, out ui.Step) error {
	kubeconfigPath, err := kubeconfigPathFor(clusterName)
	if err != nil {
		return err
	}
	kc := kubectl.New(kubeconfigPath)
	kc.Stderr = out
	kc.Logger = logf
	if err := kc.Available(); err != nil {
		return fmt.Errorf("%w; install kubectl + ensure it's on PATH", err)
	}
	return markLonghornStorageNodeWithRetry(ctx, kc, nodeName, out, 2*time.Minute, 5*time.Second)
}

type longhornStorageNodeMarker interface {
	AnnotateNode(ctx context.Context, node string, keyValue string) error
	LabelNode(ctx context.Context, node string, keyValue string) error
}

func markLonghornStorageNodeWithRetry(ctx context.Context, marker longhornStorageNodeMarker, nodeName string, out io.Writer, timeout time.Duration, interval time.Duration) error {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(timeout)
	var lastErr error
	attempt := 0
	for {
		attempt++
		err := applyLonghornStorageNodeMark(ctx, marker, nodeName)
		if err == nil {
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out marking worker node %s for Longhorn replica storage: %w", nodeName, lastErr)
		}
		if out != nil {
			fmt.Fprintf(out, "waiting for Kubernetes node %s before Longhorn storage mark (attempt %d): %v\n", nodeName, attempt, err)
		}
		wait := interval
		if remaining := time.Until(deadline); remaining < wait {
			wait = remaining
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func applyLonghornStorageNodeMark(ctx context.Context, marker longhornStorageNodeMarker, nodeName string) error {
	if err := marker.AnnotateNode(ctx, nodeName, longhornStorageNodeTagsAnnotation); err != nil {
		return err
	}
	return marker.LabelNode(ctx, nodeName, longhornStorageNodeLabel)
}

func detectBootstrapAPIHost(client *remote.Client) (string, error) {
	script := strings.Join([]string{
		`host="$(ip -4 route get 1.1.1.1 2>/dev/null | sed -n 's/.* src \([0-9.]*\).*/\1/p' | head -n1)"`,
		`if [ -z "$host" ]; then host="$(hostname -I 2>/dev/null | awk '{print $1}')"; fi`,
		`printf '%s\n' "$host"`,
	}, "; ")
	out, err := client.Capture(script)
	if err != nil {
		return "", fmt.Errorf("detect bootstrap API host: %w", err)
	}
	host := strings.TrimSpace(string(out))
	if parsed := net.ParseIP(host); parsed == nil || parsed.To4() == nil {
		return "", fmt.Errorf("detected bootstrap API host %q is not an IPv4 address; pass --bootstrap-api-host", host)
	}
	return host, nil
}

func harvestBootstrapCredentials(ctx context.Context, client *remote.Client, cfg clusterconfig.Config, clusterName string) error {
	logf("waiting for node to reboot and accept SSH")
	if err := sleepCtx(ctx, 10*time.Second); err != nil {
		return err
	}
	if err := client.WaitForAuthCtx(ctx, 5*time.Minute); err != nil {
		return err
	}

	logf("waiting for k3s credentials on bootstrap node")
	if err := waitForK3sCredentials(ctx, client, 5*time.Minute); err != nil {
		return err
	}

	logf("reading kubeconfig and k3s tokens from bootstrap node")
	rawKubeconfig, err := client.ReadSudoFile("/etc/rancher/k3s/k3s.yaml")
	if err != nil {
		return err
	}
	serverToken, err := client.ReadSudoFile("/var/lib/rancher/k3s/server/token")
	if err != nil {
		return err
	}
	nodeToken, err := client.ReadSudoFile("/var/lib/rancher/k3s/server/node-token")
	if err != nil {
		return err
	}
	agentToken, err := client.ReadSudoFile("/var/lib/rancher/k3s/server/agent-token")
	if err != nil {
		return err
	}

	rewrittenKubeconfig, err := kubeconfig.RewriteServer(rawKubeconfig, cfg.APIServerURL())
	if err != nil {
		return err
	}

	dir, err := clusterCredentialsDir(clusterName)
	if err != nil {
		return err
	}
	logf("writing cluster credentials to %s", dir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	files := map[string][]byte{
		"kubeconfig":   rewrittenKubeconfig,
		"server-token": []byte(strings.TrimSpace(string(serverToken)) + "\n"),
		"node-token":   []byte(strings.TrimSpace(string(nodeToken)) + "\n"),
		"agent-token":  []byte(strings.TrimSpace(string(agentToken)) + "\n"),
		"server-url":   []byte(cfg.APIServerURL() + "\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return err
		}
	}
	successf("cluster credentials written; use KUBECONFIG=%s", filepath.Join(dir, "kubeconfig"))
	return nil
}

func waitForK3sCredentials(ctx context.Context, client *remote.Client, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		err := client.Check(strings.Join([]string{
			"sudo test -s /etc/rancher/k3s/k3s.yaml",
			"sudo test -s /var/lib/rancher/k3s/server/token",
			"sudo test -s /var/lib/rancher/k3s/server/node-token",
			"sudo test -s /var/lib/rancher/k3s/server/agent-token",
		}, " && "))
		if err == nil {
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for k3s credentials: %w", lastErr)
		}
		if err := sleepCtx(ctx, 5*time.Second); err != nil {
			return err
		}
	}
}

func applyRootArgoApp(ctx context.Context, client *remote.Client, timeout time.Duration) error {
	logf("applying root Argo CD app manifest")
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		err := client.Run(rootArgoAppApplyScript(remoteRootArgoAppManifestPath))
		if err == nil {
			successf("root Argo CD app manifest applied")
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out applying root Argo CD app manifest: %w", lastErr)
		}
		if err := sleepCtx(ctx, 5*time.Second); err != nil {
			return err
		}
	}
}

func verifyRemoteProvisioning(ctx context.Context, client *remote.Client, description string, script string, timeout time.Duration) error {
	logf("verifying %s provisioning", description)
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		err := client.Run(script)
		if err == nil {
			successf("%s provisioning verified", description)
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out verifying %s provisioning: %w", description, lastErr)
		}
		if err := sleepCtx(ctx, 5*time.Second); err != nil {
			return err
		}
	}
}

func hardenRemoteDefaultAccess(client *remote.Client) error {
	logf("hardening default kairos access")
	script := strings.Join([]string{
		"set -eu",
		"if sudo test -f /oem/90_custom.yaml; then sudo mv /oem/90_custom.yaml /oem/90_custom.yaml.k2-disabled; fi",
		"sudo passwd -l kairos",
		"sudo test ! -e /oem/90_custom.yaml",
	}, "\n")
	if err := client.Run(script); err != nil {
		return fmt.Errorf("harden default kairos access: %w", err)
	}
	successf("default kairos access hardened")
	return nil
}

// sleepCtx sleeps for d or until ctx is cancelled, so Ctrl-C actually stops
// the multi-minute post-reboot retry loops instead of letting them run out
// their full deadlines.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}
