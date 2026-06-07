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