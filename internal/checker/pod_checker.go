package checker

import (
	"context"
	"fmt"

	promclient "github.com/your-org/k8s-health-operator/internal/prometheus"
)

type PodFinding struct {
	Namespace string
	PodName   string
	Reason    string
	Severity  string
	Message   string
}

type PodChecker struct {
	prometheus promclient.Client
}

func NewPodChecker(prometheus promclient.Client) *PodChecker {
	return &PodChecker{
		prometheus: prometheus,
	}
}

func (c *PodChecker) CheckCriticalErrorMetric(ctx context.Context, query string) ([]PodFinding, error) {
	result, err := c.prometheus.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query prometheus for pod critical errors: %w", err)
	}

	findings := make([]PodFinding, 0)

	for _, item := range result.Data.Result {
		namespace := item.Metric["namespace"]
		podName := item.Metric["pod"]

		if namespace == "" || podName == "" {
			continue
		}

		findings = append(findings, PodFinding{
			Namespace: namespace,
			PodName:   podName,
			Reason:    "CriticalErrorMetric",
			Severity:  item.Metric["severity"],
			Message:   "critical error metric detected from Prometheus",
		})
	}

	return findings, nil
}
