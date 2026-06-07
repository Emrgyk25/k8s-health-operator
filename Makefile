CLUSTER_NAME=sre-case

.PHONY: kind-create
kind-create:
	kind create cluster --config deploy/cluster/kind-config.yaml

.PHONY: kind-delete
kind-delete:
	kind delete cluster --name $(CLUSTER_NAME)

.PHONY: namespaces
namespaces:
	kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace sre-system --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace test-app --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace logging --dry-run=client -o yaml | kubectl apply -f -

.PHONY: cluster-status
cluster-status:
	kubectl get nodes -o wide
	kubectl get pods -A

.PHONY: prometheus-install
prometheus-install:
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts || true
	helm repo update
	helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
		--namespace monitoring \
		--values deploy/prometheus/values.yaml

.PHONY: prometheus-ui
prometheus-ui:
	kubectl port-forward svc/monitoring-kube-prometheus-prometheus 9090:9090 -n monitoring

.PHONY: grafana-ui
grafana-ui:
	kubectl port-forward svc/monitoring-grafana 3000:80 -n monitoring