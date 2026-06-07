package remediation

import (
	"context"
	"fmt"
	"time"

	"github.com/your-org/k8s-health-operator/internal/checker"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type PodRestarter struct {
	client ctrlclient.Client
}

func NewPodRestarter(client ctrlclient.Client) *PodRestarter {
	return &PodRestarter{
		client: client,
	}
}

func (r *PodRestarter) DeletePod(ctx context.Context, finding checker.PodFinding) error {
	if finding.Namespace == "" || finding.PodName == "" {
		return fmt.Errorf("namespace and pod name are required")
	}

	if isProtectedNamespace(finding.Namespace) {
		return fmt.Errorf("refusing to delete pod in protected namespace: %s", finding.Namespace)
	}

	pod := &corev1.Pod{}
	key := types.NamespacedName{
		Namespace: finding.Namespace,
		Name:      finding.PodName,
	}

	if err := r.client.Get(ctx, key, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get pod before delete: %w", err)
	}

	if pod.Labels["sre.io/self-healing"] != "enabled" {
		return fmt.Errorf("refusing to delete pod %s/%s because self-healing label is not enabled", finding.Namespace, finding.PodName)
	}

	gracePeriod := int64(30)
	deleteOptions := &ctrlclient.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	}

	if err := r.client.Delete(ctx, pod, deleteOptions); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("delete pod %s/%s: %w", finding.Namespace, finding.PodName, err)
	}

	return nil
}

func isProtectedNamespace(namespace string) bool {
	protected := map[string]bool{
		"kube-system":     true,
		"kube-public":     true,
		"kube-node-lease": true,
		"monitoring":      true,
		"logging":         true,
		"sre-system":      true,
	}

	return protected[namespace]
}

type CooldownStore struct {
	lastAction map[string]time.Time
	cooldown   time.Duration
}

func NewCooldownStore(cooldown time.Duration) *CooldownStore {
	return &CooldownStore{
		lastAction: make(map[string]time.Time),
		cooldown:   cooldown,
	}
}

func (s *CooldownStore) ShouldSkip(namespace, podName, reason string) bool {
	key := namespace + "/" + podName + "/" + reason

	last, exists := s.lastAction[key]
	if !exists {
		return false
	}

	return time.Since(last) < s.cooldown
}

func (s *CooldownStore) Mark(namespace, podName, reason string) {
	key := namespace + "/" + podName + "/" + reason
	s.lastAction[key] = time.Now()
}
