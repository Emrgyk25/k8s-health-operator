package remediation

import (
	"context"
	"fmt"
	"strings"

	srev1alpha1 "github.com/your-org/k8s-health-operator/api/v1alpha1"
	"github.com/your-org/k8s-health-operator/internal/checker"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type EventQueue struct {
	client    ctrlclient.Client
	namespace string
}

func NewEventQueue(client ctrlclient.Client, namespace string) *EventQueue {
	return &EventQueue{
		client:    client,
		namespace: namespace,
	}
}

func (q *EventQueue) CreateOrUpdate(ctx context.Context, finding checker.NodeFinding) error {
	if q.namespace == "" {
		return fmt.Errorf("queue event namespace cannot be empty")
	}

	name := buildQueueEventName(finding)

	existing := &srev1alpha1.QueueEvent{}
	err := q.client.Get(ctx, types.NamespacedName{
		Namespace: q.namespace,
		Name:      name,
	}, existing)

	if err == nil {
		existing.Spec.Message = finding.Message
		existing.Spec.Severity = finding.Severity
		existing.Spec.Status = "Pending"

		if updateErr := q.client.Update(ctx, existing); updateErr != nil {
			return fmt.Errorf("update queue event: %w", updateErr)
		}

		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get queue event: %w", err)
	}

	event := &srev1alpha1.QueueEvent{}
	event.APIVersion = "sre.example.com/v1alpha1"
	event.Kind = "QueueEvent"
	event.Name = name
	event.Namespace = q.namespace
	event.Spec = srev1alpha1.QueueEventSpec{
		NodeName: finding.NodeName,
		Reason:   finding.Reason,
		Severity: finding.Severity,
		Message:  finding.Message,
		Status:   "Pending",
	}

	if createErr := q.client.Create(ctx, event); createErr != nil {
		return fmt.Errorf("create queue event: %w", createErr)
	}

	return nil
}

func buildQueueEventName(finding checker.NodeFinding) string {
	base := strings.ToLower(finding.Reason + "-" + finding.NodeName)

	replacer := strings.NewReplacer(
		"_", "-",
		".", "-",
		":", "-",
		"/", "-",
	)

	name := replacer.Replace(base)

	if len(name) > 63 {
		name = name[:63]
	}

	return name
}
