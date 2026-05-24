package clusterconfig

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

const apiServerPort = 6443

type Config struct {
	ID         string     `yaml:"id"`
	ApexDomain string     `yaml:"apexDomain"`
	AWS        AWS        `yaml:"aws"`
	Kubernetes Kubernetes `yaml:"kubernetes"`
	Argo       Argo       `yaml:"argo"`
}

type AWS struct {
	AccountID  string     `yaml:"accountId"`
	Region     string     `yaml:"region"`
	OIDCIssuer OIDCIssuer `yaml:"oidcIssuer"`
}

type OIDCIssuer struct {
	URL     string `yaml:"url"`
	JWKSURI string `yaml:"jwksUri"`
}

type Kubernetes struct {
	API     string  `yaml:"api"`
	DNS     string  `yaml:"dns"`
	Domain  string  `yaml:"domain"`
	Subnets Subnets `yaml:"subnets"`
}

type Subnets struct {
	Pods     string `yaml:"pods"`
	Services string `yaml:"services"`
}

type Argo struct {
	Namespace  string `yaml:"namespace"`
	Project    string `yaml:"project"`
	RepoURL    string `yaml:"repoUrl"`
	RepoBranch string `yaml:"repoBranch"`
}

func Load(repoRoot string, target string) (Config, error) {
	path := filepath.Join(repoRoot, "clusters", target+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read cluster config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse cluster config %s: %w", path, err)
	}
	if err := cfg.validate(path, target); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) DeployDir(repoRoot string) string {
	return filepath.Join(repoRoot, "deploy")
}

func (c Config) APIServerURL() string {
	return fmt.Sprintf("https://%s:%d", c.Kubernetes.API, apiServerPort)
}

var (
	accountIDPattern = regexp.MustCompile(`^\d{12}$`)
	cidrPattern      = regexp.MustCompile(`^(?:\d{1,3}\.){3}\d{1,3}/\d{1,2}$`)
)

func (c Config) validate(path string, target string) error {
	if err := c.validateIdentity(path, target); err != nil {
		return err
	}
	if err := c.AWS.validate(path + ".aws"); err != nil {
		return err
	}
	if err := c.Kubernetes.validate(path + ".kubernetes"); err != nil {
		return err
	}
	return c.Argo.validate(path + ".argo")
}

func (c Config) validateIdentity(path string, target string) error {
	if c.ID == "" {
		return fmt.Errorf("%s: id is required", path)
	}
	if c.ID != target {
		return fmt.Errorf("%s: id %q does not match cluster target %q", path, c.ID, target)
	}
	return nil
}

func (a AWS) validate(fieldPath string) error {
	if a.AccountID != "" && !accountIDPattern.MatchString(a.AccountID) {
		return fmt.Errorf("%s.accountId: must be a 12-digit AWS account id", fieldPath)
	}
	if a.OIDCIssuer.URL != "" || a.OIDCIssuer.JWKSURI != "" {
		if err := requireHTTPSURL(a.OIDCIssuer.URL, fieldPath+".oidcIssuer.url"); err != nil {
			return err
		}
		if err := requireHTTPSURL(a.OIDCIssuer.JWKSURI, fieldPath+".oidcIssuer.jwksUri"); err != nil {
			return err
		}
	}
	return nil
}

func (k Kubernetes) validate(fieldPath string) error {
	if err := requireIPv4(k.API, fieldPath+".api"); err != nil {
		return err
	}
	if err := requireIPv4(k.DNS, fieldPath+".dns"); err != nil {
		return err
	}
	if k.Domain == "" {
		return fmt.Errorf("%s.domain: must not be empty", fieldPath)
	}
	if err := requireCIDR(k.Subnets.Pods, fieldPath+".subnets.pods"); err != nil {
		return err
	}
	if err := requireCIDR(k.Subnets.Services, fieldPath+".subnets.services"); err != nil {
		return err
	}
	return nil
}

func (a Argo) validate(fieldPath string) error {
	if a.Namespace == "" {
		return fmt.Errorf("%s.namespace: must not be empty", fieldPath)
	}
	if a.Project == "" {
		return fmt.Errorf("%s.project: must not be empty", fieldPath)
	}
	if a.RepoURL == "" {
		return fmt.Errorf("%s.repoUrl: must not be empty", fieldPath)
	}
	if a.RepoBranch == "" {
		return fmt.Errorf("%s.repoBranch: must not be empty", fieldPath)
	}
	return nil
}

func requireIPv4(value string, fieldPath string) error {
	if value == "" {
		return fmt.Errorf("%s: must not be empty", fieldPath)
	}
	parsed := net.ParseIP(value)
	if parsed == nil || parsed.To4() == nil {
		return fmt.Errorf("%s: %q is not an IPv4 address", fieldPath, value)
	}
	return nil
}

func requireCIDR(value string, fieldPath string) error {
	if value == "" {
		return fmt.Errorf("%s: must not be empty", fieldPath)
	}
	if !cidrPattern.MatchString(value) {
		return fmt.Errorf("%s: %q is not CIDR notation", fieldPath, value)
	}
	if _, _, err := net.ParseCIDR(value); err != nil {
		return fmt.Errorf("%s: %w", fieldPath, err)
	}
	return nil
}

func requireHTTPSURL(value string, fieldPath string) error {
	if value == "" {
		return fmt.Errorf("%s: must not be empty", fieldPath)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%s: must be a valid https:// URL", fieldPath)
	}
	return nil
}
