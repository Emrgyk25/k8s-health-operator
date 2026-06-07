package main

import (
	"context"
	"fmt"
	"os"

	"github.com/your-org/k8s-health-operator/internal/checker"
	promclient "github.com/your-org/k8s-health-operator/internal/prometheus"
)

func main() {
	prom := promclient.NewHTTPClient("http://localhost:9090")
	nodeChecker := checker.NewNodeChecker(prom)

	findings, err := nodeChecker.CheckNodeLimitMetric(
		context.Background(),
		`100 * (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) > 85`,
	)
	if err != nil {
		fmt.Println("check error:", err)
		os.Exit(1)
	}

	fmt.Printf("findings: %d\n", len(findings))

	for _, finding := range findings {
		fmt.Printf(
			"node=%s reason=%s severity=%s message=%s\n",
			finding.NodeName,
			finding.Reason,
			finding.Severity,
			finding.Message,
		)
	}
}
