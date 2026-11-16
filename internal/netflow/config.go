package netflow

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version  int      `yaml:"version"`
	Defaults Defaults `yaml:"defaults"`
	Flows    []Flow   `yaml:"flows"`
}
type Defaults struct {
	Timeout string `yaml:"timeout"`
	Retries int    `yaml:"retries"`
}
type Expect struct {
	Status []int `yaml:"status"`
}
type Flow struct {
	ID        string `yaml:"id" json:"id"`
	Group     string `yaml:"group" json:"group,omitempty"`
	Direction string `yaml:"direction" json:"direction,omitempty"`
	Vantage   string `yaml:"vantage" json:"vantage,omitempty"`
	Protocol  string `yaml:"protocol" json:"protocol"`
	Host      string `yaml:"host" json:"host,omitempty"`
	Port      int    `yaml:"port" json:"port,omitempty"`
	URL       string `yaml:"url" json:"url,omitempty"`
	Expect    Expect `yaml:"expect" json:"-"`
}

func Load(path string) (Config, time.Duration, error) {
	var c Config
	f, err := os.Open(path)
	if err != nil {
		return c, 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return c, 0, err
	}
	if info.Size() > 1<<20 {
		return c, 0, fmt.Errorf("config exceeds 1 MiB")
	}
	d := yaml.NewDecoder(io.LimitReader(f, 1<<20))
	d.KnownFields(true)
	if err = d.Decode(&c); err != nil {
		return c, 0, err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		return c, 0, fmt.Errorf("expected exactly one YAML document")
	}
	t, err := c.Validate()
	return c, t, err
}
func (c Config) Validate() (time.Duration, error) {
	if c.Version != 1 {
		return 0, fmt.Errorf("version must be 1")
	}
	timeout := c.Defaults.Timeout
	if timeout == "" {
		timeout = "5s"
	}
	t, err := time.ParseDuration(timeout)
	if err != nil || t <= 0 || t > 5*time.Minute {
		return 0, fmt.Errorf("timeout must be between 1ns and 5m")
	}
	if c.Defaults.Retries < 0 || c.Defaults.Retries > 5 {
		return 0, fmt.Errorf("retries must be between 0 and 5")
	}
	if len(c.Flows) == 0 {
		return 0, fmt.Errorf("flows must not be empty")
	}
	seen := map[string]bool{}
	for _, f := range c.Flows {
		if f.ID == "" || seen[f.ID] {
			return 0, fmt.Errorf("flow IDs must be nonempty and unique")
		}
		seen[f.ID] = true
		if err := f.Validate(); err != nil {
			return 0, fmt.Errorf("flow %q: %w", f.ID, err)
		}
	}
	return t, nil
}
func (f Flow) Validate() error {
	if f.Direction != "" && f.Direction != "inbound" && f.Direction != "outbound" {
		return fmt.Errorf("direction must be inbound or outbound")
	}
	switch f.Protocol {
	case "dns", "tcp", "tls":
		if strings.TrimSpace(f.Host) == "" || strings.ContainsAny(f.Host, "/ \t\n") {
			return fmt.Errorf("host must be a hostname or IP address")
		}
		if f.Protocol != "dns" && (f.Port < 1 || f.Port > 65535) {
			return fmt.Errorf("port must be between 1 and 65535")
		}
		if f.Protocol == "dns" && f.Port != 0 {
			return fmt.Errorf("dns does not accept port")
		}
		if f.URL != "" || len(f.Expect.Status) != 0 {
			return fmt.Errorf("url and expect.status require http or https")
		}
	case "http", "https":
		u, err := url.Parse(f.URL)
		if err != nil || u.Scheme != f.Protocol || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return fmt.Errorf("url must match protocol and have no credentials or fragment")
		}
		if u.Port() != "" {
			p, e := strconv.Atoi(u.Port())
			if e != nil || p < 1 || p > 65535 {
				return fmt.Errorf("URL port must be between 1 and 65535")
			}
		}
		if strings.HasSuffix(u.Host, ":") {
			return fmt.Errorf("URL port must not be empty")
		}
		if f.Host != "" || f.Port != 0 {
			return fmt.Errorf("http(s) uses url instead of host/port")
		}
		for _, s := range f.Expect.Status {
			if s < 100 || s > 599 {
				return fmt.Errorf("invalid expected HTTP status")
			}
		}
	default:
		return fmt.Errorf("unsupported protocol %q (use dns, tcp, tls, http, https)", f.Protocol)
	}
	return nil
}
