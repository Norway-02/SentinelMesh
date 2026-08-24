package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/sentinelmesh/sentinelmesh/internal/policy"
)

// BuildNetworkPolicy constructs a Kubernetes NetworkPolicy enforcing profile egress rules.
func BuildNetworkPolicy(namespace, podName string, profile policy.SecurityProfile) *networkingv1.NetworkPolicy {
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName + "-netpol",
			Namespace: namespace,
			Labels: map[string]string{
				"app":                     "sentinelmesh-agent",
				"sentinelmesh.io/profile": string(profile.Name),
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": podName,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeEgress,
			},
		},
	}

	// If Confidential profile, zero egress rules (default deny all egress)
	if profile.Name == policy.ProfileConfidential {
		return np
	}

	var egressRules []networkingv1.NetworkPolicyEgressRule

	// Add DNS resolution rule (always needed for k8s internal operations)
	dnsPort := intstr.FromInt(53)
	udpProto := corev1.ProtocolUDP
	tcpProto := corev1.ProtocolTCP
	egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: &udpProto, Port: &dnsPort},
			{Protocol: &tcpProto, Port: &dnsPort},
		},
	})

	// Add allowed ports/CIDRs
	var npPorts []networkingv1.NetworkPolicyPort
	for _, p := range profile.Network.AllowedPorts {
		portVal := intstr.FromInt(p)
		proto := corev1.ProtocolTCP
		npPorts = append(npPorts, networkingv1.NetworkPolicyPort{
			Protocol: &proto,
			Port:     &portVal,
		})
	}

	var npPeers []networkingv1.NetworkPolicyPeer
	for _, cidr := range profile.Network.AllowedCIDRs {
		peer := networkingv1.NetworkPolicyPeer{
			IPBlock: &networkingv1.IPBlock{
				CIDR: cidr,
			},
		}
		if len(profile.Network.DeniedCIDRs) > 0 {
			peer.IPBlock.Except = profile.Network.DeniedCIDRs
		}
		npPeers = append(npPeers, peer)
	}

	if len(npPeers) > 0 || len(npPorts) > 0 {
		egressRules = append(egressRules, networkingv1.NetworkPolicyEgressRule{
			Ports: npPorts,
			To:    npPeers,
		})
	}

	np.Spec.Egress = egressRules
	return np
}
