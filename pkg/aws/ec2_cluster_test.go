package aws

import (
	"strings"
	"testing"
)

func TestNamesForEC2ClusterCapacityProviderIsValid(t *testing.T) {
	for _, clusterName := range []string{
		"my-cluster",
		"ecsctl-live-smoke-18daa2c88a6ed07c",
		"cluster name with characters that need sanitizing and a long suffix",
	} {
		names := namesForEC2Cluster(clusterName)
		if len(names.capacityProvider) == 0 || len(names.capacityProvider) > 255 {
			t.Errorf("capacity provider name length = %d for cluster %q", len(names.capacityProvider), clusterName)
		}
		for _, reserved := range []string{"aws", "ecs", "fargate"} {
			if strings.HasPrefix(strings.ToLower(names.capacityProvider), reserved) {
				t.Errorf("capacity provider %q uses reserved %q prefix", names.capacityProvider, reserved)
			}
		}
		for _, r := range names.capacityProvider {
			if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-", r) {
				t.Errorf("capacity provider %q contains invalid character %q", names.capacityProvider, r)
			}
		}
	}
}
