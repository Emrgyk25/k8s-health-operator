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

.PHONY: test-app-build
test-app-build:
	docker build -t test-app:0.1.0 ./test-app

.PHONY: test-app-load
test-app-load:
	kind load docker-image test-app:0.1.0 --name $(CLUSTER_NAME)

.PHONY: test-app-deploy
test-app-deploy:
	kubectl apply -f deploy/test-app/

.PHONY: test-app-port-forward
test-app-port-forward:
	kubectl port-forward svc/test-app 8080:8080 -n test-app

.PHONY: test-app-logs
test-app-logs:
	kubectl logs -f deployment/test-app -n test-app

.PHONY: test-app-restart
test-app-restart: test-app-build test-app-load test-app-deploy