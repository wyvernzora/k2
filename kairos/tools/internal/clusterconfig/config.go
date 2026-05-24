package clusterconfig

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"
)

const apiServerPort = 6443

type Config struct {
	ID         string     `yaml:"id"`
	ApexDomain string     `yaml:"apexDomain"`
	Kubernetes Kubernetes `yaml:"kubernetes"`
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

var cidrPattern = regexp.MustCompile(`^(?:\d{1,3}\.){3}\d{1,3}/\d{1,2}$`)

func (c Config) validate(path string, target string) error {
	if c.ID == "" {
		return fmt.Errorf("%s: id is required", path)
	}
	if c.ID != target {
		return fmt.Errorf("%s: id %q does not match cluster target %q", path, c.ID, target)
	}
	if err := requireIPv4(c.Kubernetes.API, path+".kubernetes.api"); err != nil {
		return err
	}
	if err := requireIPv4(c.Kubernetes.DNS, path+".kubernetes.dns"); err != nil {
		return err
	}
	if c.Kubernetes.Domain == "" {
		return fmt.Errorf("%s.kubernetes.domain: must not be empty", path)
	}
	if err := requireCIDR(c.Kubernetes.Subnets.Pods, path+".kubernetes.subnets.pods"); err != nil {
		return err
	}
	if err := requireCIDR(c.Kubernetes.Subnets.Services, path+".kubernetes.subnets.services"); err != nil {
		return err
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
