package netflow

import (
	"net"
	"os"
	"runtime"
	"sort"
)

// Source describes the execution environment, not the address observed after NAT.
type Source struct {
	Hostname               string            `json:"hostname"`
	OS                     string            `json:"os"`
	Architecture           string            `json:"architecture"`
	Vantage                string            `json:"vantage,omitempty"`
	Interfaces             []Interface       `json:"interfaces"`
	Kubernetes             *KubernetesSource `json:"kubernetes,omitempty"`
	ObservedSourceIP       string            `json:"observed_source_ip"`
	ObservedSourceIPStatus string            `json:"observed_source_ip_status"`
	Warnings               []string          `json:"warnings,omitempty"`
}
type Interface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
}
type KubernetesSource struct {
	PodName        string `json:"pod_name,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
	NodeName       string `json:"node_name,omitempty"`
	PodIP          string `json:"pod_ip,omitempty"`
	MetadataSource string `json:"metadata_source"`
}

func CollectSource(vantage string) Source {
	s := Source{OS: runtime.GOOS, Architecture: runtime.GOARCH, Vantage: vantage, Interfaces: []Interface{}, ObservedSourceIPStatus: "UNKNOWN"}
	name, err := os.Hostname()
	if err != nil {
		s.Warnings = append(s.Warnings, "HOSTNAME_UNAVAILABLE")
	} else {
		s.Hostname = name
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		s.Warnings = append(s.Warnings, "INTERFACES_UNAVAILABLE")
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			s.Warnings = append(s.Warnings, "INTERFACE_ADDRESSES_UNAVAILABLE")
			continue
		}
		entry := Interface{Name: iface.Name, Addresses: []string{}}
		for _, a := range addrs {
			entry.Addresses = append(entry.Addresses, a.String())
		}
		sort.Strings(entry.Addresses)
		s.Interfaces = append(s.Interfaces, entry)
	}
	sort.Slice(s.Interfaces, func(i, j int) bool { return s.Interfaces[i].Name < s.Interfaces[j].Name })
	k := KubernetesSource{PodName: os.Getenv("NETFLOW_POD_NAME"), Namespace: os.Getenv("NETFLOW_POD_NAMESPACE"), NodeName: os.Getenv("NETFLOW_NODE_NAME"), PodIP: os.Getenv("NETFLOW_POD_IP"), MetadataSource: "environment"}
	if k.PodName != "" || k.Namespace != "" || k.NodeName != "" || k.PodIP != "" {
		s.Kubernetes = &k
	}
	return s
}

type Assessment struct {
	ExpectedPathVerification string `json:"expected_path_verification,omitempty"`
	TCPConnectivity          string `json:"tcp_connectivity"`
	Authentication           string `json:"authentication"`
	FirewallVerdict          string `json:"firewall_verdict"`
	Summary                  string `json:"summary"`
}

func (r *Result) finalize() {
	expected := []string{"dns"}
	if r.Flow.Protocol != "dns" {
		expected = append(expected, "tcp")
	}
	if r.Flow.Protocol == "tls" || r.Flow.Protocol == "https" {
		expected = append(expected, "tls")
	}
	if r.Flow.Protocol == "http" || r.Flow.Protocol == "https" {
		expected = append(expected, "http")
	}
	seen := map[string]bool{}
	r.Assessment = Assessment{TCPConnectivity: "NOT_TESTED", Authentication: "NOT_TESTED", FirewallVerdict: "UNKNOWN"}
	if len(r.Flow.ExpectedPath) > 0 {
		r.Assessment.ExpectedPathVerification = "NOT_TESTED"
	}
	for _, s := range r.Stages {
		seen[s.Name] = true
		if s.Status == "FAIL" {
			r.FailedStage = s.Name
		}
		if s.Name == "tcp" {
			r.Assessment.TCPConnectivity = s.Status
		}
	}
	for _, name := range expected {
		if !seen[name] {
			r.Stages = append(r.Stages, Stage{Name: name, Status: "NOT_TESTED", Code: "NOT_REACHED"})
		}
	}
	switch {
	case r.Status == "SKIPPED":
		r.Assessment.Summary = "Not tested at this execution location."
	case r.Assessment.TCPConnectivity == "PASS" && r.Status == "PASS":
		r.Assessment.Summary = "TCP reachable; configured checks passed. Authentication not tested."
	case r.Assessment.TCPConnectivity == "PASS":
		r.Assessment.Summary = "TCP reachable; a later check failed. Authentication not tested."
	case r.Assessment.TCPConnectivity == "FAIL":
		r.Assessment.Summary = "TCP connection failed; firewall blocking is not established."
	case r.Status == "PASS":
		r.Assessment.Summary = "Name/address resolution passed; TCP and authentication not tested."
	default:
		r.Assessment.Summary = "TCP not tested; inspect the failed stage."
	}
}
