package clusterconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ID          string `yaml:"id"`
	DeployPath  string `yaml:"deployPath"`
	ApexDomain  string `yaml:"apexDomain"`
	OnePassword struct {
		VaultID string `yaml:"vaultId"`
	} `yaml:"onePassword"`
	Kubernetes Kubernetes `yaml:"kubernetes"`
}

type Kubernetes struct {
	API        API        `yaml:"api"`
	Networking Networking `yaml:"networking"`
}

type API struct {
	VIP     string   `yaml:"vip"`
	DNSName string   `yaml:"dnsName"`
	Port    int      `yaml:"port"`
	TLSSans []string `yaml:"tlsSans"`
}

type Networking struct {
	PodCIDR       string `yaml:"podCidr"`
	ServiceCIDR   string `yaml:"serviceCidr"`
	ClusterDNS    string `yaml:"clusterDns"`
	ClusterDomain string `yaml:"clusterDomain"`
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
	if c.DeployPath == "" {
		return filepath.Join(repoRoot, "deploy", c.ID)
	}
	return filepath.Join(repoRoot, c.DeployPath)
}

func (c Config) APIServerURL() string {
	port := c.Kubernetes.API.Port
	if port == 0 {
		port = 6443
	}
	host := c.Kubernetes.API.DNSName
	if host == "" {
		host = c.Kubernetes.API.VIP
	}
	return fmt.Sprintf("https://%s:%d", host, port)
}

func (c Config) APIVIPURL() string {
	port := c.Kubernetes.API.Port
	if port == 0 {
		port = 6443
	}
	return fmt.Sprintf("https://%s:%d", c.Kubernetes.API.VIP, port)
}

func (c Config) validate(path string, target string) error {
	if c.ID == "" {
		return fmt.Errorf("%s: id is required", path)
	}
	if c.ID != target {
		return fmt.Errorf("%s: id %q does not match cluster target %q", path, c.ID, target)
	}
	if c.Kubernetes.API.VIP == "" {
		return fmt.Errorf("%s: kubernetes.api.vip is required", path)
	}
	if c.Kubernetes.Networking.PodCIDR == "" {
		return fmt.Errorf("%s: kubernetes.networking.podCidr is required", path)
	}
	if c.Kubernetes.Networking.ServiceCIDR == "" {
		return fmt.Errorf("%s: kubernetes.networking.serviceCidr is required", path)
	}
	if c.Kubernetes.Networking.ClusterDNS == "" {
		return fmt.Errorf("%s: kubernetes.networking.clusterDns is required", path)
	}
	if c.Kubernetes.Networking.ClusterDomain == "" {
		return fmt.Errorf("%s: kubernetes.networking.clusterDomain is required", path)
	}
	return nil
}
