package main

import (
	"context"
	"fmt"
	"os"

	promclient "github.com/your-org/k8s-health-operator/internal/prometheus"
)

func main() {
	client := promclient.NewHTTPClient("http://localhost:9090")

	result, err := client.Query(context.Background(), `app_error_total`)
	if err != nil {
		fmt.Println("query error:", err)
		os.Exit(1)
	}

	fmt.Printf("status: %s\n", result.Status)
	fmt.Printf("result count: %d\n", len(result.Data.Result))

	for _, item := range result.Data.Result {
		fmt.Printf("metric=%v value=%v\n", item.Metric, item.Value)
	}
}