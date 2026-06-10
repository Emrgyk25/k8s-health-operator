# k8s-health-operator

`k8s-health-operator`, Prometheus metriklerini izleyerek Kubernetes pod ve node sağlık durumlarını kontrol eden, belirli hata senaryolarında otomatik aksiyon alan Go tabanlı bir Kubernetes Operator projesidir.

Temel amaç:

* Kubernetes cluster üzerinde çalışan uygulamaları izlemek
* Prometheus üzerinden pod/node metriklerini sorgulamak
* Belirli bir hata metriği oluştuğunda ilgili pod’u otomatik silmek
* Node kaynak kullanımı belirlenen eşiklere yaklaştığında queue event oluşturmak
* Uygulama ve operator loglarını Fluent Bit ile toplayıp Elasticsearch/Kibana üzerinden görüntülemek
* Grafana üzerinden metrikleri dashboard olarak izlemek

---

## İçindekiler

* [Mimari](#mimari)
* [Kullanılan Teknolojiler](#kullanılan-teknolojiler)
* [Proje Yapısı](#proje-yapısı)
* [Kurulum Ön Gereksinimleri](#kurulum-ön-gereksinimleri)
* [Sıfırdan Kurulum](#sıfırdan-kurulum)
* [Test Senaryoları](#test-senaryoları)
* [Prometheus Kullanımı](#prometheus-kullanımı)
* [Grafana Kullanımı](#grafana-kullanımı)
* [Kibana Kullanımı](#kibana-kullanımı)
* [QueueEvent Testi](#queueevent-testi)
* [Güvenlik Yaklaşımı](#güvenlik-yaklaşımı)
* [Concurrency ve Leader Election](#concurrency-ve-leader-election)
* [Bilinen Kısıtlar](#bilinen-kısıtlar)
* [Makefile Komutları](#makefile-komutları)

---

## Mimari

Genel sistem akışı aşağıdaki gibidir:

```text
test-app
  ├── /metrics endpoint'i ile Prometheus'a metric verir
  ├── /error endpoint'i ile hata metriği üretir
  └── JSON log üretir

Prometheus
  ├── test-app metriclerini toplar
  ├── Kubernetes cluster, pod, deployment ve node metriclerini toplar
  └── operator tarafından sorgulanır

k8s-health-operator
  ├── HealthPolicy CRD'sini izler
  ├── Prometheus'a PromQL query gönderir
  ├── Kritik hata metriği bulursa pod'u siler
  ├── Node kaynak kullanımı yüksekse QueueEvent oluşturur
  └── Leader election ile çoklu instance çalışmasını güvenli hale getirir

Fluent Bit
  ├── Kubernetes pod loglarını toplar
  └── Elasticsearch'e gönderir

Elasticsearch
  └── Logları k8s-logs-* indexleri altında saklar

Kibana
  └── Logların aranması ve incelenmesi için kullanılır

Grafana
  └── Prometheus metriclerini dashboard olarak gösterir
```

Basit mimari diyagram:

```text
┌────────────────────┐
│      test-app      │
│ /error  /metrics   │
└─────────┬──────────┘
          │
          │ metrics
          ▼
┌────────────────────┐
│     Prometheus     │
│ app + k8s metrics  │
└─────────┬──────────┘
          │ PromQL
          ▼
┌─────────────────────────────┐
│    k8s-health-operator      │
│ HealthPolicy Controller     │
│ PodChecker / NodeChecker    │
│ PodRestarter / EventQueue   │
└───────┬─────────────┬───────┘
        │             │
        │ delete pod  │ create QueueEvent
        ▼             ▼
┌──────────────┐   ┌──────────────┐
│ Kubernetes   │   │ QueueEvent   │
│ Deployment   │   │ CRD          │
└──────────────┘   └──────────────┘


┌────────────────────┐
│ Kubernetes Logs    │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│    Fluent Bit      │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│  Elasticsearch     │
└─────────┬──────────┘
          │
          ▼
┌────────────────────┐
│      Kibana        │
└────────────────────┘
```

---

## Kullanılan Teknolojiler

| Bileşen               | Amaç                                                            |
| --------------------- | --------------------------------------------------------------- |
| kind                  | Lokal Kubernetes cluster oluşturmak                             |
| Kubernetes            | Uygulama ve operator runtime ortamı                             |
| Go                    | Operator ve test uygulaması geliştirme dili                     |
| controller-runtime    | Kubernetes operator/controller geliştirme                       |
| Prometheus            | Metric toplama ve sorgulama                                     |
| kube-prometheus-stack | Prometheus, Grafana, kube-state-metrics, node-exporter kurulumu |
| Grafana               | Prometheus metriclerini görselleştirme                          |
| ECK                   | Elasticsearch ve Kibana yönetimi                                |
| Elasticsearch         | Log saklama ve indexleme                                        |
| Kibana                | Log arama ve inceleme arayüzü                                   |
| Fluent Bit            | Kubernetes pod loglarını toplama                                |
| Docker                | Image build işlemleri                                           |
| Makefile              | Kurulum ve operasyon komutlarını kolaylaştırma                  |

Not: Bu projede klasik Logstash tabanlı ELK yerine **EFK benzeri** bir yapı kullanılmıştır:

```text
Fluent Bit + Elasticsearch + Kibana
```

Fluent Bit, Logstash yerine kullanılmıştır çünkü Kubernetes DaemonSet olarak hafif ve pratik log forwarding sağlar.

---

## Proje Yapısı

```text
k8s-health-operator/
├── api/
│   └── v1alpha1/
│       ├── healthpolicy_types.go
│       ├── queueevent_types.go
│       ├── groupversion_info.go
│       └── zz_generated.deepcopy.go
│
├── cmd/
│   ├── manager/
│   │   └── main.go
│   ├── checkertest/
│   ├── nodetest/
│   └── promtest/
│
├── internal/
│   ├── checker/
│   │   ├── pod_checker.go
│   │   └── node_checker.go
│   ├── controller/
│   │   └── healthpolicy_controller.go
│   ├── prometheus/
│   │   └── client.go
│   └── remediation/
│       ├── pod_restarter.go
│       └── event_queue.go
│
├── deploy/
│   ├── cluster/
│   │   └── kind-config.yaml
│   ├── prometheus/
│   │   └── values.yaml
│   ├── operator/
│   │   ├── namespace.yaml
│   │   ├── healthpolicy-crd.yaml
│   │   ├── queueevent-crd.yaml
│   │   ├── serviceaccount.yaml
│   │   ├── clusterrole.yaml
│   │   ├── clusterrolebinding.yaml
│   │   ├── deployment.yaml
│   │   └── healthpolicy-sample.yaml
│   ├── test-app/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   └── servicemonitor.yaml
│   ├── elk/
│   │   ├── elasticsearch.yaml
│   │   └── kibana.yaml
│   └── fluent-bit/
│       ├── serviceaccount.yaml
│       ├── clusterrole.yaml
│       ├── clusterrolebinding.yaml
│       ├── configmap.yaml
│       └── daemonset.yaml
│
├── test-app/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
│
├── Dockerfile
├── Makefile
├── README.md
└── README.tr.md
```

---

## Kurulum Ön Gereksinimleri

macOS üzerinde aşağıdaki araçlar gereklidir:

```bash
brew install kind kubectl helm
```

Docker Desktop kurulu ve açık olmalıdır.

Kontrol:

```bash
docker version
kind version
kubectl version --client
helm version
```

---

## Sıfırdan Kurulum

### 1. Proje klasörüne gir

```bash
cd ~/Desktop/k8s-health-operator
```

---

### 2. Elasticsearch ve Kibana versiyonunu kontrol et

ECK 2.14.0 ile `9.0.0` Elasticsearch versiyonu desteklenmediği için manifestlerde `8.15.0` kullanılmalıdır.

Kontrol:

```bash
grep -R "version:" deploy/elk/
```

Beklenen:

```text
deploy/elk/elasticsearch.yaml:  version: 8.15.0
deploy/elk/kibana.yaml:  version: 8.15.0
```

Eğer `9.0.0` görürsen düzelt:

```bash
sed -i '' 's/version: 9.0.0/version: 8.15.0/g' deploy/elk/elasticsearch.yaml
sed -i '' 's/version: 9.0.0/version: 8.15.0/g' deploy/elk/kibana.yaml
```

---

### 3. kind cluster oluştur

```bash
make kind-create
```

Kontrol:

```bash
kubectl get nodes
```

Beklenen:

```text
sre-case-control-plane   Ready
sre-case-worker          Ready
sre-case-worker2         Ready
```

---

### 4. Namespace’leri oluştur

```bash
make namespaces
```

Oluşması gereken namespace’ler:

```text
monitoring
sre-system
test-app
logging
```

Kontrol:

```bash
kubectl get ns
```

---

### 5. Prometheus ve Grafana kurulumu

```bash
make prometheus-install
```

Kontrol:

```bash
kubectl get pods -n monitoring
```

Beklenen podlar:

```text
monitoring-grafana-...
monitoring-kube-prometheus-operator-...
monitoring-kube-state-metrics-...
monitoring-prometheus-node-exporter-...
prometheus-monitoring-kube-prometheus-prometheus-0
```

---

### 6. ECK kurulumu

```bash
make eck-install
```

ECK operator hazır olana kadar bekle:

```bash
kubectl wait --for=condition=Ready pod \
  -l control-plane=elastic-operator \
  -n elastic-system --timeout=180s
```

Kontrol:

```bash
kubectl get pods -n elastic-system
```

Beklenen:

```text
elastic-operator-xxxxx   1/1   Running
```

---

### 7. Elasticsearch ve Kibana kurulumu

```bash
kubectl apply -f deploy/elk/elasticsearch.yaml
kubectl apply -f deploy/elk/kibana.yaml
```

Podları izle:

```bash
kubectl get pods -n logging -w
```

Beklenen:

```text
sre-logs-es-default-0      1/1   Running
sre-kibana-kb-xxxxx        1/1   Running
```

Elasticsearch hazır olana kadar beklemek için:

```bash
kubectl wait --for=condition=Ready pod/sre-logs-es-default-0 \
  -n logging --timeout=300s
```

Kontrol:

```bash
kubectl get elasticsearch -n logging
kubectl get kibana -n logging
```

Tek node Elasticsearch ortamında `yellow` health normaldir.

---

### 8. Fluent Bit kurulumu

Elasticsearch hazır olduktan sonra Fluent Bit kurulmalıdır:

```bash
kubectl apply -f deploy/fluent-bit/
```

Kontrol:

```bash
kubectl get pods -n logging
```

Beklenen:

```text
fluent-bit-xxxxx   1/1   Running
fluent-bit-yyyyy   1/1   Running
fluent-bit-zzzzz   1/1   Running
```

Log kontrolü:

```bash
kubectl logs -f daemonset/fluent-bit -n logging
```

---

### 9. Test uygulamasını deploy et

```bash
make test-app-build
make test-app-load
make test-app-deploy
```

Kontrol:

```bash
kubectl get pods -n test-app
kubectl get svc -n test-app
kubectl get servicemonitor -n test-app
```

Beklenen:

```text
test-app-xxxxx   1/1   Running
service/test-app
servicemonitor/test-app
```

Pod label kontrolü:

```bash
kubectl get pods -n test-app --show-labels
```

Podlarda şu label olmalıdır:

```text
sre.io/self-healing=enabled
```

Bu label olmayan podlara operator müdahale etmez.

---

### 10. Operator build/load/deploy

```bash
make operator-build
make operator-load
make operator-deploy
```

Kontrol:

```bash
kubectl get crd | grep -E "healthpolicies|queueevents"
kubectl get healthpolicy -n sre-system
kubectl get pods -n sre-system
```

Beklenen:

```text
healthpolicies.sre.example.com
queueevents.sre.example.com

default-health-policy

k8s-health-operator-xxxxx   1/1   Running
k8s-health-operator-yyyyy   1/1   Running
```

---

## HealthPolicy Örneği

`HealthPolicy`, operator’ün hangi Prometheus query’lerini çalıştıracağını ve hangi aksiyonları alacağını tanımlar.

Örnek:

```yaml
apiVersion: sre.example.com/v1alpha1
kind: HealthPolicy
metadata:
  name: default-health-policy
  namespace: sre-system
spec:
  checkIntervalSeconds: 30
  remediationCooldownSeconds: 300
  prometheusUrl: http://monitoring-kube-prometheus-prometheus.monitoring.svc:9090
  targetNamespaces:
    - test-app
  podRules:
    - name: restart-on-critical-error
      query: 'app_error_total{namespace="test-app",severity="critical",error_code="POD_RESTART_REQUIRED"} > 0'
      action: DeletePod
  nodeRules:
    - name: node-memory-high
      query: '100 * (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) > 85'
      action: CreateQueueEvent
      threshold: "85"
```

Demo için kullanılan pod query:

```promql
app_error_total{namespace="test-app",severity="critical",error_code="POD_RESTART_REQUIRED"} > 0
```

Daha gerçekçi production query örneği:

```promql
increase(app_error_total{namespace="test-app",severity="critical",error_code="POD_RESTART_REQUIRED"}[5m]) > 0
```

---

## Test Senaryoları

### 1. Test app health kontrolü

Port-forward aç:

```bash
kubectl port-forward svc/test-app 8080:8080 -n test-app
```

Başka terminalde:

```bash
curl http://localhost:8080/health
```

Beklenen:

```json
{"status":"healthy"}
```

Ready kontrolü:

```bash
curl http://localhost:8080/ready
```

Beklenen:

```json
{"status":"ready"}
```

---

### 2. Hata metriği üretme

```bash
curl http://localhost:8080/error
```

Beklenen response:

```json
{
  "error_code": "POD_RESTART_REQUIRED",
  "message": "simulated critical error",
  "severity": "critical",
  "status": "error"
}
```

Metric kontrolü:

```bash
curl http://localhost:8080/metrics | grep app_error_total
```

Beklenen:

```text
app_error_total{error_code="POD_RESTART_REQUIRED",severity="critical"} 1
```

---

### 3. Prometheus metric kontrolü

Prometheus port-forward:

```bash
kubectl port-forward svc/monitoring-kube-prometheus-prometheus 9091:9090 -n monitoring
```

Metric sorgusu:

```bash
curl -sG "http://localhost:9091/api/v1/query" \
  --data-urlencode 'query=app_error_total' | jq '.data.result[].metric'
```

Beklenen label’lar:

```json
{
  "__name__": "app_error_total",
  "container": "test-app",
  "endpoint": "http",
  "error_code": "POD_RESTART_REQUIRED",
  "instance": "10.244.x.x:8080",
  "job": "test-app",
  "namespace": "test-app",
  "pod": "test-app-xxxxx",
  "service": "test-app",
  "severity": "critical"
}
```

HealthPolicy query’sini birebir test etmek için:

```bash
curl -sG "http://localhost:9091/api/v1/query" \
  --data-urlencode 'query=app_error_total{namespace="test-app",severity="critical",error_code="POD_RESTART_REQUIRED"} > 0' | jq '.data.result'
```

Sonuç boş dönmemelidir.

---

### 4. Pod remediation testi

Bir terminalde podları izle:

```bash
kubectl get pods -n test-app -w
```

Başka terminalde hata üret:

```bash
curl http://localhost:8080/error
```

Beklenen davranış:

```text
test-app-xxxxx   1/1   Terminating
test-app-yyyyy   0/1   Pending
test-app-yyyyy   0/1   ContainerCreating
test-app-yyyyy   1/1   Running
```

Bu akış şu anlama gelir:

```text
1. test-app /error endpoint'i hata metriği üretti.
2. Prometheus bu metriği scrape etti.
3. k8s-health-operator Prometheus'u sorguladı.
4. HealthPolicy pod rule eşleşti.
5. Operator ilgili pod'u sildi.
6. Kubernetes Deployment yeni pod oluşturdu.
```

Operator log kontrolü:

```bash
kubectl logs deployment/k8s-health-operator -n sre-system --tail=150
```

Beklenen loglardan biri:

```text
deleting pod because critical metric detected
```

Cooldown aktifse:

```text
skipping pod remediation because cooldown is active
```

---

## Prometheus Kullanımı

Prometheus’u açmak için:

```bash
kubectl port-forward svc/monitoring-kube-prometheus-prometheus 9091:9090 -n monitoring
```

Tarayıcıdan:

```text
http://localhost:9091
```

Örnek PromQL sorguları:

```promql
app_error_total
```

```promql
app_error_total{namespace="test-app",severity="critical",error_code="POD_RESTART_REQUIRED"}
```

```promql
increase(app_error_total{namespace="test-app",severity="critical",error_code="POD_RESTART_REQUIRED"}[5m])
```

```promql
test_app_http_requests_total
```

```promql
sum by (path, status) (test_app_http_requests_total{namespace="test-app"})
```

```promql
kube_pod_container_status_restarts_total{namespace="test-app"}
```

```promql
sum(kube_pod_status_phase{namespace="test-app", phase="Running"})
```

```promql
100 * (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes))
```

```promql
100 - (avg by(instance)(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
```

---

## Grafana Kullanımı

Grafana’yı açmak için:

```bash
kubectl port-forward svc/monitoring-grafana 3000:80 -n monitoring
```

Tarayıcıdan:

```text
http://localhost:3000
```

Kullanıcı adı:

```text
admin
```

Şifreyi almak için:

```bash
kubectl get secret monitoring-grafana -n monitoring \
  -o jsonpath="{.data.admin-password}" | base64 --decode
echo
```

Eğer Helm values içinde `adminPassword: admin` tanımlıysa:

```text
admin / admin
```

Grafana’da dashboard oluşturmak için:

```text
Dashboards → New → New dashboard → Add visualization
```

Data source olarak Prometheus seçilir.

Önerilen paneller:

### Critical Application Errors

```promql
app_error_total{namespace="test-app",severity="critical",error_code="POD_RESTART_REQUIRED"}
```

### Error Increase Last 5 Minutes

```promql
increase(app_error_total{namespace="test-app",severity="critical",error_code="POD_RESTART_REQUIRED"}[5m])
```

### HTTP Requests by Path and Status

```promql
sum by (path, status) (test_app_http_requests_total{namespace="test-app"})
```

### Running Test App Pods

```promql
sum(kube_pod_status_phase{namespace="test-app", phase="Running"})
```

### Pod Restarts

```promql
kube_pod_container_status_restarts_total{namespace="test-app"}
```

### Node Memory Usage

```promql
100 * (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes))
```

### Node CPU Usage

```promql
100 - (avg by(instance)(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)
```

Not: Grafana sadece görselleştirme katmanıdır. Operator kararlarını Grafana üzerinden almaz. Operator doğrudan Prometheus API’sini sorgular.

---

## Kibana Kullanımı

Kibana’yı açmak için:

```bash
kubectl port-forward svc/sre-kibana-kb-http 5601:5601 -n logging
```

Tarayıcıdan:

```text
https://localhost:5601
```

Kullanıcı adı:

```text
elastic
```

Şifre:

```bash
kubectl get secret sre-logs-es-elastic-user -n logging \
  -o jsonpath='{.data.elastic}' | base64 --decode
echo
```

### Data View Oluşturma

Kibana’da:

```text
Stack Management → Data Views → Create data view
```

Değerler:

```text
Index pattern: k8s-logs-*
Time field: @timestamp
```

Sonra:

```text
Discover
```

ekranına girilir.

Aranabilecek örnekler:

```text
POD_RESTART_REQUIRED
```

```text
kubernetes.namespace_name : "test-app"
```

```text
kubernetes.namespace_name : "sre-system"
```

```text
"deleting pod because critical metric detected"
```

```text
"skipping pod remediation because cooldown is active"
```

---

## Elasticsearch Kontrolü

Elasticsearch API port-forward:

```bash
kubectl port-forward svc/sre-logs-es-http 9200:9200 -n logging
```

Başka terminalde:

```bash
export ELASTIC_PASSWORD=$(kubectl get secret sre-logs-es-elastic-user -n logging -o=jsonpath='{.data.elastic}' | base64 --decode)

curl -k -u "elastic:${ELASTIC_PASSWORD}" "https://localhost:9200/_cat/indices?v"
```

Beklenen index:

```text
k8s-logs-YYYY.MM.DD
```

Log sayısı kontrolü:

```bash
curl -k -u "elastic:${ELASTIC_PASSWORD}" "https://localhost:9200/k8s-logs-*/_count?pretty"
```

---

## QueueEvent Testi

Node resource threshold normalde `%85` olarak ayarlanmıştır:

```promql
100 * (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) > 85
```

Lokal kind ortamında bu eşiği gerçekçi şekilde tetiklemek zor olabilir. Test için geçici olarak threshold `%1` yapılabilir:

```bash
kubectl patch healthpolicy default-health-policy \
  -n sre-system \
  --type='json' \
  -p='[
    {
      "op": "replace",
      "path": "/spec/nodeRules/0/query",
      "value": "100 * (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) > 1"
    }
  ]'
```

Bir süre bekledikten sonra:

```bash
kubectl get queueevents -n sre-system
kubectl describe queueevent -n sre-system
```

Beklenen:

```text
noderesourcelimit-...
```

Test bittikten sonra query eski değerine çekilmelidir:

```bash
kubectl patch healthpolicy default-health-policy \
  -n sre-system \
  --type='json' \
  -p='[
    {
      "op": "replace",
      "path": "/spec/nodeRules/0/query",
      "value": "100 * (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) > 85"
    }
  ]'
```

QueueEvent temizlemek için:

```bash
kubectl delete queueevents --all -n sre-system
```

---

## Güvenlik Yaklaşımı

Projede aşağıdaki güvenlik yaklaşımları uygulanmıştır:

| Güvenlik Önlemi           | Açıklama                                                                                                 |
| ------------------------- | -------------------------------------------------------------------------------------------------------- |
| Opt-in remediation        | Sadece `sre.io/self-healing=enabled` label’ı olan podlara müdahale edilir                                |
| Protected namespaces      | `kube-system`, `kube-public`, `monitoring`, `logging`, `sre-system` gibi namespace’lere müdahale edilmez |
| Minimal RBAC              | Operator sadece ihtiyaç duyduğu kaynaklar için yetkilendirilir                                           |
| Non-root container        | Operator container non-root user ile çalışır                                                             |
| Read-only root filesystem | Operator deployment üzerinde read-only root filesystem kullanılabilir                                    |
| Distroless image          | Minimal ve shell içermeyen image kullanılır                                                              |
| TLS log forwarding        | Fluent Bit, Elasticsearch’e TLS ve authentication ile log gönderir                                       |
| Cooldown                  | Aynı pod için tekrarlı müdahale engellenir                                                               |

---

## Concurrency ve Leader Election

Operator birden fazla replica ile çalışacak şekilde tasarlanmıştır.

Deployment 2 replica olarak çalıştırılabilir:

```bash
kubectl scale deployment/k8s-health-operator -n sre-system --replicas=2
```

Leader election sayesinde aynı anda sadece bir operator instance reconcile işlemi yapar.

Leader kontrolü:

```bash
kubectl get lease -n sre-system
```

Leader holder bilgisini görmek için:

```bash
kubectl get lease -n sre-system k8s-health-operator.sre.example.com \
  -o jsonpath='{.spec.holderIdentity}{"\n"}'
```

Bu yaklaşım aynı pod üzerinde iki farklı operator instance’ın aynı anda işlem yapmasını engeller.

---

## Bilinen Kısıtlar

* `CooldownStore` in-memory olarak tutulur. Operator restart olursa cooldown bilgisi sıfırlanır.
* `HealthPolicy.status` alanları temel seviyede güncellenir, ancak production seviyesinde daha detaylı status yönetimi yapılabilir.
* `targetNamespaces` alanı CRD’de vardır, ancak MVP’de namespace filtreleme çoğunlukla PromQL query üzerinden yapılmaktadır.
* `QueueEvent` objeleri oluşturulur, ancak bunları işleyen ayrı bir QueueEvent controller henüz yoktur.
* `zz_generated.deepcopy.go` MVP kapsamında manuel olarak eklenmiş olabilir. Production projelerde `controller-gen` ile üretilmesi önerilir.
* Lokal kind ortamında tek node Elasticsearch kullanıldığı için index health `yellow` olabilir.
* Demo için kullanılan query `app_error_total > 0` gibi agresif davranabilir. Production senaryoda `increase(...[5m]) > 0` gibi daha kontrollü query kullanılması önerilir.

---

## Makefile Komutları

| Komut                        | Açıklama                                                    |
| ---------------------------- | ----------------------------------------------------------- |
| `make kind-create`           | kind cluster oluşturur                                      |
| `make kind-delete`           | kind cluster siler                                          |
| `make namespaces`            | Gerekli namespace’leri oluşturur                            |
| `make cluster-status`        | Cluster node ve pod durumlarını gösterir                    |
| `make prometheus-install`    | kube-prometheus-stack kurar                                 |
| `make prometheus-ui`         | Prometheus port-forward açar                                |
| `make grafana-ui`            | Grafana port-forward açar                                   |
| `make eck-install`           | ECK operator kurar                                          |
| `make logging-deploy`        | Elasticsearch, Kibana ve Fluent Bit manifestlerini uygular  |
| `make kibana-ui`             | Kibana port-forward açar                                    |
| `make elastic-password`      | Elasticsearch elastic kullanıcısı şifresini gösterir        |
| `make test-app-build`        | test-app Docker image build eder                            |
| `make test-app-load`         | test-app image’ını kind cluster’a yükler                    |
| `make test-app-deploy`       | test-app manifestlerini uygular                             |
| `make test-app-port-forward` | test-app için port-forward açar                             |
| `make test-app-logs`         | test-app loglarını izler                                    |
| `make operator-build`        | operator Docker image build eder                            |
| `make operator-load`         | operator image’ını kind cluster’a yükler                    |
| `make operator-deploy`       | operator CRD, RBAC, deployment ve sample HealthPolicy kurar |
| `make operator-logs`         | operator loglarını izler                                    |
| `make setup`                 | temel altyapı kurulumunu yapar                              |

---

## Kısa Demo Akışı

Sistemin uçtan uca çalıştığını göstermek için:

```bash
kubectl port-forward svc/test-app 8080:8080 -n test-app
```

Başka terminalde podları izle:

```bash
kubectl get pods -n test-app -w
```

Üçüncü terminalde hata üret:

```bash
curl http://localhost:8080/error
```

Prometheus metric kontrolü:

```bash
curl -sG "http://localhost:9091/api/v1/query" \
  --data-urlencode 'query=app_error_total{namespace="test-app",severity="critical",error_code="POD_RESTART_REQUIRED"} > 0' | jq '.data.result'
```

Operator log kontrolü:

```bash
kubectl logs deployment/k8s-health-operator -n sre-system --tail=150
```

Elasticsearch index kontrolü:

```bash
export ELASTIC_PASSWORD=$(kubectl get secret sre-logs-es-elastic-user -n logging -o=jsonpath='{.data.elastic}' | base64 --decode)

curl -k -u "elastic:${ELASTIC_PASSWORD}" "https://localhost:9200/_cat/indices?v"
```

Kibana’da aranacak değer:

```text
POD_RESTART_REQUIRED
```

---

## Sonuç

Bu proje, Prometheus metriklerine dayalı otomatik Kubernetes remediation akışını uçtan uca göstermektedir.

Özet akış:

```text
/error endpoint'i hata metriği üretir
Prometheus metriği toplar
k8s-health-operator HealthPolicy üzerinden metriği sorgular
Kritik hata metriği varsa ilgili pod silinir
Deployment yeni pod oluşturur
Fluent Bit JSON logları Elasticsearch'e gönderir
Kibana logları gösterir
Grafana metrikleri dashboard olarak gösterir
```
