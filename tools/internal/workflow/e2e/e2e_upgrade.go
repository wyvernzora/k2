package e2e

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wyvernzora/k2/tools/internal/client/remote"
	"github.com/wyvernzora/k2/tools/internal/ui"
)

// e2eIdentityFacts are the node-owned values an image upgrade must not
// disturb.
//
// Assert them as a LIST, not one at a time. The bug this guards is a class —
// image content landing on top of node state during an upgrade — and it has
// already produced two distinct instances: the ZFS hostid baked per image
// build (fixed 2026-08-10) and the iSCSI IQN baked into every k8s image
// (fixed 2026-08-11, after two nodes silently adopted their image's copy).
// The next instance will be some other file in the same class, so the cheap
// defence is to keep widening this list rather than to test one file well.
var e2eIdentityFacts = []struct{ name, command string }{
	{"iqn", `sed -n 's/^InitiatorName=//p' /etc/iscsi/initiatorname.iscsi 2>/dev/null | head -n1`},
	{"iqn-state", `cat /usr/local/.state/k2-iscsi-iqn 2>/dev/null`},
	{"hostid", `od -An -tx1 /etc/hostid 2>/dev/null | tr -d ' \n'`},
	{"machine-id", `cat /etc/machine-id 2>/dev/null`},
	{"ssh-host-key", `ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub 2>/dev/null | awk '{print $2}'`},
}

// e2eIdentityRequired names facts that must be non-empty BEFORE the upgrade.
// Without this the whole check passes vacuously: an unreadable path yields ""
// on both sides and compares equal, so a broken capture would look like a
// perfectly preserved identity.
var e2eIdentityRequired = []string{"iqn", "machine-id"}

type e2eImageUpgradeStep struct {
	VM     string `yaml:"vm"`
	Source string `yaml:"source"`
}

func e2eCaptureIdentityScript() string {
	var buf strings.Builder
	buf.WriteString("set -u\n")
	for _, fact := range e2eIdentityFacts {
		fmt.Fprintf(&buf, "printf '%s=%%s\\n' \"$(%s)\"\n", fact.name, fact.command)
	}
	return buf.String()
}

func parseE2EIdentityFacts(out string) map[string]string {
	facts := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		name, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found || name == "" {
			continue
		}
		facts[name] = value
	}
	return facts
}

// diffE2EIdentityFacts reports every fact whose value moved, sorted so the
// failure message is stable.
func diffE2EIdentityFacts(before, after map[string]string) []string {
	var changed []string
	for name, was := range before {
		if now := after[name]; now != was {
			changed = append(changed, fmt.Sprintf("%s: %q -> %q", name, was, now))
		}
	}
	sort.Strings(changed)
	return changed
}

func missingE2EIdentityFacts(facts map[string]string) []string {
	var missing []string
	for _, name := range e2eIdentityRequired {
		if strings.TrimSpace(facts[name]) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

// stepE2EImageUpgrade upgrades a booted node to the image under test and
// asserts its identity survived. Upgrading to the image the node ALREADY runs
// is a legitimate configuration and still exercises the failure path: what
// broke was the new image's /etc landing on top of persisted state, which
// happens whether or not the two builds differ.
func stepE2EImageUpgrade(s *e2eScenarioState, step e2eImageUpgradeStep, sourceOverride string) func(context.Context, ui.Step) error {
	return func(ctx context.Context, sh ui.Step) error {
		source := firstNonEmpty(sourceOverride, step.Source)
		if strings.TrimSpace(source) == "" {
			return fmt.Errorf("imageUpgrade: no source image configured for %q", step.VM)
		}
		target, ok := s.targets[step.VM]
		if !ok {
			return fmt.Errorf("imageUpgrade: no reachable target recorded for %q", step.VM)
		}
		id := s.vmIDs[step.VM]
		client := remote.Client{Host: target.Host, Port: target.Port, User: "kairos", IdentityFile: s.operatorPriv, InsecureHostKey: true, NoPasswordPrompt: true, Stdout: sh, Stderr: sh, Logger: shLogf(sh)}

		out, err := client.Capture(e2eCaptureIdentityScript())
		if err != nil {
			return fmt.Errorf("capture identity on %s: %w", id, err)
		}
		before := parseE2EIdentityFacts(string(out))
		if missing := missingE2EIdentityFacts(before); len(missing) > 0 {
			return fmt.Errorf("%s reported no value for %s before the upgrade; the check would pass vacuously", id, strings.Join(missing, ", "))
		}
		sh.Successf("captured %d identity facts on %s", len(before), id)

		if err := client.Run(fmt.Sprintf("sudo kairos-agent upgrade --source %s", shellQuote(source))); err != nil {
			return fmt.Errorf("upgrade %s to %s: %w", id, source, err)
		}
		if err := client.RunAllowDisconnect("sudo reboot"); err != nil {
			return fmt.Errorf("reboot %s: %w", id, err)
		}
		if err := sleepCtx(ctx, 10*time.Second); err != nil {
			return err
		}
		client.ResetAuth()
		if err := client.WaitForAuthCtx(ctx, 10*time.Minute); err != nil {
			return err
		}
		mode, err := client.Capture("if [ -e /run/cos/recovery_mode ] || [ -e /run/cos/autoreset_mode ]; then echo recovery; else echo active; fi")
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(mode)) != "active" {
			return fmt.Errorf("%s booted into recovery/autoreset after upgrading to %s", id, source)
		}

		out, err = client.Capture(e2eCaptureIdentityScript())
		if err != nil {
			return fmt.Errorf("re-capture identity on %s: %w", id, err)
		}
		after := parseE2EIdentityFacts(string(out))
		if changed := diffE2EIdentityFacts(before, after); len(changed) > 0 {
			return fmt.Errorf("%s changed node identity across the upgrade to %s: %s", id, source, strings.Join(changed, "; "))
		}
		sh.Successf("%s upgraded to %s with all %d identity facts intact", id, source, len(before))
		return nil
	}
}
