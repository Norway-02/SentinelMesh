package scheduler

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sentinelmesh/sentinelmesh/internal/domain"
)

type KubernetesResourceProvider struct {
	client client.Client
}

func NewKubernetesResourceProvider(c client.Client) *KubernetesResourceProvider {
	return &KubernetesResourceProvider{
		client: c,
	}
}

func (p *KubernetesResourceProvider) ListNodes(ctx context.Context) ([]domain.Node, error) {
	var nodeList corev1.NodeList
	if err := p.client.List(ctx, &nodeList); err != nil {
		return nil, fmt.Errorf("failed to list kubernetes nodes: %w", err)
	}

	var nodes []domain.Node
	for _, k8sNode := range nodeList.Items {
		nodes = append(nodes, p.mapToDomainNode(&k8sNode))
	}

	return nodes, nil
}

func (p *KubernetesResourceProvider) mapToDomainNode(k8sNode *corev1.Node) domain.Node {
	allocatable := k8sNode.Status.Allocatable
	capacity := k8sNode.Status.Capacity

	cpuAllocQty := allocatable[corev1.ResourceCPU]
	memAllocQty := allocatable[corev1.ResourceMemory]
	cpuCapQty := capacity[corev1.ResourceCPU]
	memCapQty := capacity[corev1.ResourceMemory]
	
	// Convert CPU from resource.Quantity (e.g. "2000m", "2") to float64 (e.g. 2.0)
	cpuAllocFloat := float64(cpuAllocQty.MilliValue()) / 1000.0
	cpuCapFloat := float64(cpuCapQty.MilliValue()) / 1000.0
	if cpuCapFloat == 0 && cpuAllocFloat > 0 {
		cpuCapFloat = cpuAllocFloat
	}

	// Convert Memory from bytes to MB
	memAllocFloat := float64(memAllocQty.Value()) / (1024 * 1024)
	memCapFloat := float64(memCapQty.Value()) / (1024 * 1024)
	if memCapFloat == 0 && memAllocFloat > 0 {
		memCapFloat = memAllocFloat
	}

	// Determine node health (check Ready condition)
	health := domain.NodeHealthUnhealthy
	for _, condition := range k8sNode.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			health = domain.NodeHealthHealthy
			break
		}
	}

	// Map taints
	var taints []string
	for _, t := range k8sNode.Spec.Taints {
		taints = append(taints, fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect))
	}

	// Security class could be mapped from a label, fallback to "standard"
	securityClass := k8sNode.Labels["sentinelmesh.io/security-class"]
	if securityClass == "" {
		securityClass = "standard"
	}

	// Cost could be mapped from a label or instance type, fallback to 1.0
	cost := 1.0

	slog.Debug("Mapped Kubernetes node to SentinelMesh node", 
		"k8s_node", k8sNode.Name, 
		"cpu_alloc", cpuAllocFloat,
		"cpu_cap", cpuCapFloat, 
		"mem_alloc_mb", memAllocFloat,
		"mem_cap_mb", memCapFloat,
		"health", health)

	return domain.Node{
		ID:            k8sNode.Name,
		ClusterID:     "local",
		Labels:        k8sNode.Labels,
		Taints:        taints,
		SecurityClass: securityClass,
		Health:        health,
		CostPerHour:   cost,
		Resources: domain.NodeResources{
			CPUCapacity:     cpuCapFloat,
			CPUAvailable:    cpuAllocFloat, // Starts from allocatable capacity
			MemoryCapacity:  memCapFloat,
			MemoryAvailable: memAllocFloat, // Starts from allocatable capacity
			GPUCapacity:     0,             // K8s GPU integration hook for future stages
			GPUAvailable:    0,
		},
	}
}
