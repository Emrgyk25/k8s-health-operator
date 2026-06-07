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
	podChecker := checker.NewPodChecker(prom)

	findings, err := podChecker.CheckCriticalErrorMetric(
		context.Background(),
		`increase(app_error_total{namespace="test-app",severity="critical",error_code="POD_RESTART_REQUIRED"}[2m]) > 0`,
	)
	if err != nil {
		fmt.Println("check error:", err)
		os.Exit(1)
	}

	fmt.Printf("findings: %d\n", len(findings))

	for _, finding := range findings {
		fmt.Printf(
			"namespace=%s pod=%s reason=%s severity=%s message=%s\n",
			finding.Namespace,
			finding.PodName,
			finding.Reason,
			finding.Severity,
			finding.Message,
		)
	}
}
