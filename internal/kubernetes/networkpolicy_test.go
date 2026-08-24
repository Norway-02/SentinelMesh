package kubernetes

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"

	"github.com/sentinelmesh/sentinelmesh/internal/policy"
)

func TestBuildNetworkPolicy_Profiles(t *testing.T) {
	profiles := policy.DefaultProfiles()

	// 1. Confidential Profile
	npConf := BuildNetworkPolicy("sentinelmesh", "agent-001", profiles[policy.ProfileConfidential])
	if npConf == nil {
		t.Fatal("expected non-nil NetworkPolicy")
	}
	if len(npConf.Spec.Egress) != 0 {
		t.Errorf("expected 0 egress rules for confidential profile (strict default deny), got %d", len(npConf.Spec.Egress))
	}
	if npConf.Spec.PolicyTypes[0] != networkingv1.PolicyTypeEgress {
		t.Errorf("expected Egress policy type, got %v", npConf.Spec.PolicyTypes)
	}

	// 2. Standard Profile
	npStd := BuildNetworkPolicy("sentinelmesh", "agent-002", profiles[policy.ProfileStandard])
	if len(npStd.Spec.Egress) == 0 {
		t.Errorf("expected egress rules for standard profile, got 0")
	}

	// 3. Restricted Profile
	npRest := BuildNetworkPolicy("sentinelmesh", "agent-003", profiles[policy.ProfileRestricted])
	if len(npRest.Spec.Egress) == 0 {
		t.Errorf("expected egress rules for restricted profile, got 0")
	}
}
