package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

func (in *HealthPolicy) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}

	out := new(HealthPolicy)
	*out = *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()

	if in.Spec.TargetNamespaces != nil {
		out.Spec.TargetNamespaces = append([]string{}, in.Spec.TargetNamespaces...)
	}

	if in.Spec.PodRules != nil {
		out.Spec.PodRules = append([]PodRule{}, in.Spec.PodRules...)
	}

	if in.Spec.NodeRules != nil {
		out.Spec.NodeRules = append([]NodeRule{}, in.Spec.NodeRules...)
	}

	return out
}

func (in *HealthPolicyList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}

	out := new(HealthPolicyList)
	*out = *in

	if in.Items != nil {
		out.Items = make([]HealthPolicy, len(in.Items))
		copy(out.Items, in.Items)
	}

	return out
}

func (in *QueueEvent) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}

	out := new(QueueEvent)
	*out = *in
	out.ObjectMeta = *in.ObjectMeta.DeepCopy()

	return out
}

func (in *QueueEventList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}

	out := new(QueueEventList)
	*out = *in

	if in.Items != nil {
		out.Items = make([]QueueEvent, len(in.Items))
		copy(out.Items, in.Items)
	}

	return out
}
