package conf

import (
	"encoding/json"
	"fmt"
	"log"
	"net/netip"
	"os"
	"sync"
)

var (
	cfg  *Config
	once sync.Once
)

// Get config Info
func Info() *Config {
	if cfg == nil {
		panic("config not loaded: call df.LoadConfig(path) first")
	}
	return cfg
}

func LoadConfig(path string) {
	once.Do(func() {
		c, err := load(path)
		if err != nil {
			log.Fatalf("failed load config error: %v\n", err)
		}
		cfg = c
	})
}

func load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file %q: %w", path, err)
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config file %q failed: %w", path, err)
	}

	// validation
	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("validation failed for config file %q: %w", path, err)
	}

	// apply default config
	applyDefault(&cfg)

	return &cfg, nil
}

func validate(cfg *Config) error {

	if cfg.Options.Bootstrap != "" {
		_, err := netip.ParseAddr(cfg.Options.Bootstrap)
		if err != nil {
			return fmt.Errorf("bootstrap must be a single IP address, got %q: %w", cfg.Options.Bootstrap, err)
		}
	}
	if cfg.Server.Standard == 0 {
		return fmt.Errorf("config port.standard is required")
	}
	if cfg.Panel.Port == 0 {
		return fmt.Errorf("config port.panel is required")
	}

	return nil
}

func applyDefault(cfg *Config) {
	if cfg.Options.Bootstrap == "" {
		cfg.Options.Bootstrap = "8.8.8.8"
	}
}

type Config struct {
	Panel    PanelConfig  `json:"panel"`
	TLS      TLSConfig    `json:"tls"`
	Server   ServerConfig `json:"server"`
	Upstream []string     `json:"upstream"`
	Options  Options      `json:"options"`
	Block    BlockConfig  `json:"block"`
	Log      LogConfig    `json:"log"`
}

type PanelConfig struct {
	Port int             `json:"port"`
	Auth AuthPanelConfig `json:"auth"`
}

type AuthPanelConfig struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

type TLSConfig struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

type ServerConfig struct {
	Standard int        `json:"standard"` // TCP+UDP
	Dot      int        `json:"dot"`      // DNS over TLS
	Doq      int        `json:"doq"`      // DNS over QUIC
	Doh      DohConfig  `json:"doh"`      // DNS over HTTPS
	HTTP     HttpConfig `json:"http"`     // DNS over HTTP
}

type DohConfig struct {
	Port int           `json:"port"`
	Path string        `json:"path"`
	Auth []AuthDohItem `json:"auth"`
}
type AuthDohItem struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

type HttpConfig struct {
	Port int    `json:"port"`
	Path string `json:"path"`
}

type Options struct {
	TTL         TTLOptions `json:"ttl"`
	EDNS0Subnet string     `json:"edns0_subnet"`
	Policy      int        `json:"policy"`
	Bootstrap   string     `json:"bootstrap"`
}

type TTLOptions struct {
	Max       int `json:"max"`
	Min       int `json:"min"`
	Overwrite int `json:"overwrite"` // 重写 TTL
}

type BlockConfig struct {
	Domain        []string `json:"domain"`
	DomainSuffix  []string `json:"domain_suffix"`
	ClientAddress []string `json:"client_address"`
	RuleSet       []string `json:"rule_set"`
}

type LogConfig struct {
	Level string `json:"level"`
}
