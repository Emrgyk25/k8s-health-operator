package controller

import (
	"context"
	"fmt"
	"time"

	srev1alpha1 "github.com/your-org/k8s-health-operator/api/v1alpha1"
	"github.com/your-org/k8s-health-operator/internal/checker"
	promclient "github.com/your-org/k8s-health-operator/internal/prometheus"
	"github.com/your-org/k8s-health-operator/internal/remediation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type HealthPolicyReconciler struct {
	ctrlclient.Client
	Scheme *runtime.Scheme

	Cooldown *remediation.CooldownStore
}

func (r *HealthPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	policy := &srev1alpha1.HealthPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("get healthpolicy: %w", err)
	}

	checkInterval := time.Duration(policy.Spec.CheckIntervalSeconds) * time.Second
	if checkInterval <= 0 {
		checkInterval = 30 * time.Second
	}

	prometheusURL := policy.Spec.PrometheusURL
	if prometheusURL == "" {
		prometheusURL = "http://monitoring-kube-prometheus-prometheus.monitoring.svc:9090"
	}

	prom := promclient.NewHTTPClient(prometheusURL)

	podChecker := checker.NewPodChecker(prom)
	podRestarter := remediation.NewPodRestarter(r.Client)

	for _, rule := range policy.Spec.PodRules {
		if rule.Action != "DeletePod" {
			continue
		}

		findings, err := podChecker.CheckCriticalErrorMetric(ctx, rule.Query)
		if err != nil {
			log.Error(err, "pod rule check failed", "rule", rule.Name)
			continue
		}

		for _, finding := range findings {
			if r.Cooldown != nil && r.Cooldown.ShouldSkip(finding.Namespace, finding.PodName, finding.Reason) {
				log.Info("skipping pod remediation because cooldown is active",
					"namespace", finding.Namespace,
					"pod", finding.PodName,
					"reason", finding.Reason,
				)
				continue
			}

			log.Info("deleting pod because critical metric detected",
				"namespace", finding.Namespace,
				"pod", finding.PodName,
				"reason", finding.Reason,
			)

			if err := podRestarter.DeletePod(ctx, finding); err != nil {
				log.Error(err, "pod remediation failed",
					"namespace", finding.Namespace,
					"pod", finding.PodName,
				)
				continue
			}

			if r.Cooldown != nil {
				r.Cooldown.Mark(finding.Namespace, finding.PodName, finding.Reason)
			}
		}
	}

	nodeChecker := checker.NewNodeChecker(prom)
	eventQueue := remediation.NewEventQueue(r.Client, policy.Namespace)

	for _, rule := range policy.Spec.NodeRules {
		if rule.Action != "CreateQueueEvent" {
			continue
		}

		findings, err := nodeChecker.CheckNodeLimitMetric(ctx, rule.Query)
		if err != nil {
			log.Error(err, "node rule check failed", "rule", rule.Name)
			continue
		}

		for _, finding := range findings {
			log.Info("creating queue event because node limit detected",
				"node", finding.NodeName,
				"reason", finding.Reason,
			)

			if err := eventQueue.CreateOrUpdate(ctx, finding); err != nil {
				log.Error(err, "queue event creation failed",
					"node", finding.NodeName,
				)
				continue
			}
		}
	}

	return ctrl.Result{
		RequeueAfter: checkInterval,
	}, nil
}

func (r *HealthPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&srev1alpha1.HealthPolicy{}).
		Complete(r)
}
