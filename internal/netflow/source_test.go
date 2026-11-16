package netflow

import "testing"

func TestSourceMetadata(t *testing.T) {
	for _, key := range []string{"NETFLOW_POD_NAME", "NETFLOW_POD_NAMESPACE", "NETFLOW_NODE_NAME", "NETFLOW_POD_IP"} {
		t.Setenv(key, "")
	}
	s := CollectSource("vm")
	if s.Kubernetes != nil || s.OS == "" || s.Architecture == "" || s.ObservedSourceIP != "" || s.ObservedSourceIPStatus != "UNKNOWN" {
		t.Fatalf("%+v", s)
	}
	t.Setenv("NETFLOW_POD_NAME", "probe")
	t.Setenv("NETFLOW_POD_NAMESPACE", "apps")
	t.Setenv("NETFLOW_NODE_NAME", "worker")
	t.Setenv("NETFLOW_POD_IP", "10.0.0.2")
	s = CollectSource("aks")
	if s.Kubernetes == nil || s.Kubernetes.PodName != "probe" || s.Kubernetes.MetadataSource != "environment" || s.Vantage != "aks" {
		t.Fatalf("%+v", s)
	}
}
