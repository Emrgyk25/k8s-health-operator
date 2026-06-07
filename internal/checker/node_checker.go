package checker

import (
	"context"
	"fmt"

	promclient "github.com/your-org/k8s-health-operator/internal/prometheus"
)

type NodeFinding struct {
	NodeName string
	Reason   string
	Severity string
	Message  string
}

type NodeChecker struct {
	prometheus promclient.Client
}

func NewNodeChecker(prometheus promclient.Client) *NodeChecker {
	return &NodeChecker{
		prometheus: prometheus,
	}
}

func (c *NodeChecker) CheckNodeLimitMetric(ctx context.Context, query string) ([]NodeFinding, error) {
	result, err := c.prometheus.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query prometheus for node limits: %w", err)
	}

	findings := make([]NodeFinding, 0)

	for _, item := range result.Data.Result {
		nodeName := item.Metric["node"]

		if nodeName == "" {
			nodeName = item.Metric["instance"]
		}

		if nodeName == "" {
			nodeName = "unknown-node"
		}

		findings = append(findings, NodeFinding{
			NodeName: nodeName,
			Reason:   "NodeResourceLimit",
			Severity: "warning",
			Message:  "node resource usage is above configured threshold",
		})
	}

	return findings, nil
}
