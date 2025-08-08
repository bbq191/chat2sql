# ☸️ Kubernetes云原生部署指南

## 🎯 技术概述

Kubernetes 1.31+为Chat2SQL提供了企业级容器编排能力，通过云原生架构实现高可用、高扩展、高安全的生产环境部署。本指南详细介绍P5阶段第1周的Kubernetes部署策略。

### ✨ 核心价值

| 功能特性 | 技术实现 | 业务价值 | 性能提升 |
|---------|---------|---------|---------| 
| **容器编排** | Kubernetes 1.31+ | 资源利用率优化 | 利用率提升60% |
| **服务网格** | Istio 1.24+ mTLS | 零信任网络安全 | 安全事件减少95% |
| **自动扩缩容** | HPA/VPA/KEDA | 成本与性能平衡 | 成本节省40% |
| **GitOps部署** | ArgoCD/Flux | 部署效率提升 | 部署时间减少80% |

### 🎁 业务场景

- **高可用部署**：多节点集群、故障自动转移、零停机更新
- **弹性扩缩容**：基于负载自动扩缩容、节约成本
- **安全隔离**：命名空间隔离、网络策略、RBAC权限控制
- **DevOps自动化**：CI/CD集成、基础设施即代码

---

## 🏗️ Kubernetes 1.31+集群架构

### 📦 集群规划设计

```yaml
# Chat2SQL生产集群规划
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-architecture
  namespace: kube-system
data:
  cluster-spec: |
    集群配置:
      name: chat2sql-prod
      version: v1.31.2
      region: us-west-2
      availability-zones: [us-west-2a, us-west-2b, us-west-2c]
      
    节点配置:
      control-plane:
        count: 3
        instance-type: t3.large
        disk: 100GB SSD
      worker-nodes:
        count: 6
        instance-type: t3.xlarge  
        disk: 200GB SSD
        auto-scaling: true
        min-nodes: 3
        max-nodes: 20
        
    网络配置:
      cni: cilium
      service-cidr: 10.96.0.0/12
      pod-cidr: 10.244.0.0/16
      network-policy: enabled
      
    存储配置:
      csi-driver: aws-ebs-csi-driver
      storage-classes:
        - name: gp3-ssd
          provisioner: ebs.csi.aws.com
          parameters:
            type: gp3
            encrypted: "true"
```

### 🎯 Kubernetes 1.31+新特性应用

```yaml
# Gateway API v1.1 - 替代传统Ingress
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: chat2sql-gateway
  namespace: chat2sql
spec:
  gatewayClassName: istio
  listeners:
  - name: http
    port: 80
    protocol: HTTP
    hostname: "chat2sql.example.com"
  - name: https
    port: 443
    protocol: HTTPS
    hostname: "chat2sql.example.com"
    tls:
      mode: Terminate
      certificateRefs:
      - name: chat2sql-tls
---
# HTTPRoute配置
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: chat2sql-routes
  namespace: chat2sql
spec:
  parentRefs:
  - name: chat2sql-gateway
  hostnames:
  - "chat2sql.example.com"
  rules:
  - matches:
    - path:
        type: PathPrefix
        value: /api/
    backendRefs:
    - name: chat2sql-backend
      port: 8080
  - matches:
    - path:
        type: PathPrefix
        value: /
    backendRefs:
    - name: chat2sql-frontend
      port: 3000
```

### 📊 JobSet API用于批处理任务

```yaml
# JobSet API (Stable in 1.31) - SQL批处理分析
apiVersion: jobset.x-k8s.io/v1alpha2
kind: JobSet
metadata:
  name: sql-batch-analysis
  namespace: chat2sql
spec:
  replicatedJobs:
  - name: data-processor
    replicas: 3
    template:
      spec:
        parallelism: 5
        completions: 15
        template:
          metadata:
            labels:
              app: sql-batch-processor
          spec:
            restartPolicy: OnFailure
            containers:
            - name: processor
              image: chat2sql/batch-processor:v1.0.0
              env:
              - name: BATCH_SIZE
                value: "1000"
              - name: PROCESSING_MODE
                value: "parallel"
              resources:
                requests:
                  memory: "1Gi"
                  cpu: "500m"
                limits:
                  memory: "2Gi"
                  cpu: "1000m"
  - name: result-aggregator
    replicas: 1
    template:
      spec:
        template:
          spec:
            containers:
            - name: aggregator
              image: chat2sql/result-aggregator:v1.0.0
              env:
              - name: INPUT_SOURCE
                value: "data-processor-results"
```

---

## 🕸️ Istio服务网格配置

### 📦 Istio 1.24+部署

```bash
# Istio安装脚本
#!/bin/bash
set -e

echo "🚀 开始安装Istio 1.24+"

# 下载Istio
curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.24.0 sh -
cd istio-1.24.0
export PATH=$PWD/bin:$PATH

# 安装Istio控制平面
istioctl install --set values.defaultRevision=default -y

# 启用自动注入
kubectl label namespace chat2sql istio-injection=enabled

# 验证安装
echo "✅ 验证Istio安装"
istioctl verify-install

echo "🎉 Istio安装完成"
```

### 🔒 mTLS配置和零信任网络

```yaml
# 全局mTLS策略
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: chat2sql
spec:
  mtls:
    mode: STRICT
---
# 目标规则 - 强制mTLS
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: chat2sql-mtls
  namespace: chat2sql
spec:
  host: "*.chat2sql.svc.cluster.local"
  trafficPolicy:
    tls:
      mode: ISTIO_MUTUAL
---
# 授权策略 - 细粒度访问控制
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
  name: chat2sql-authz
  namespace: chat2sql
spec:
  selector:
    matchLabels:
      app: chat2sql-backend
  rules:
  - from:
    - source:
        principals: ["cluster.local/ns/chat2sql/sa/chat2sql-frontend"]
  - to:
    - operation:
        methods: ["GET", "POST"]
        paths: ["/api/*"]
```

### 🌐 流量管理配置

```yaml
# 虚拟服务 - 智能路由
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: chat2sql-vs
  namespace: chat2sql
spec:
  hosts:
  - chat2sql.example.com
  gateways:
  - chat2sql-gateway
  http:
  # AI查询路由 - 高优先级
  - match:
    - uri:
        prefix: /api/ai/
    route:
    - destination:
        host: chat2sql-ai-service
        port:
          number: 8080
    timeout: 30s
    retries:
      attempts: 3
      perTryTimeout: 10s
  # 普通API路由
  - match:
    - uri:
        prefix: /api/
    route:
    - destination:
        host: chat2sql-backend
        port:
          number: 8080
    fault:
      delay:
        percentage:
          value: 0.1
        fixedDelay: 5s
  # 前端路由
  - match:
    - uri:
        prefix: /
    route:
    - destination:
        host: chat2sql-frontend
        port:
          number: 3000
---
# 目标规则 - 负载均衡和熔断
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: chat2sql-backend-dr
  namespace: chat2sql
spec:
  host: chat2sql-backend
  trafficPolicy:
    loadBalancer:
      consistentHash:
        httpHeaderName: "user-id"
    connectionPool:
      tcp:
        maxConnections: 100
      http:
        http1MaxPendingRequests: 50
        maxRequestsPerConnection: 2
    circuitBreaker:
      consecutiveGatewayErrors: 5
      interval: 30s
      baseEjectionTime: 30s
      maxEjectionPercent: 50
```

---

## 💾 存储配置与管理

### 📦 CSI驱动配置

```yaml
# AWS EBS CSI Driver StorageClass
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: chat2sql-ssd
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: ebs.csi.aws.com
parameters:
  type: gp3
  iops: "3000"
  throughput: "125"
  encrypted: "true"
  kmsKeyId: "arn:aws:kms:us-west-2:123456789012:key/12345678-1234-1234-1234-123456789012"
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
---
# PostgreSQL持久化存储
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgresql-pvc
  namespace: chat2sql
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: chat2sql-ssd
  resources:
    requests:
      storage: 100Gi
---
# Redis持久化存储
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: redis-pvc
  namespace: chat2sql
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: chat2sql-ssd
  resources:
    requests:
      storage: 20Gi
```

### 🔄 StatefulSet配置

```yaml
# PostgreSQL StatefulSet
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgresql
  namespace: chat2sql
spec:
  serviceName: postgresql
  replicas: 3
  selector:
    matchLabels:
      app: postgresql
  template:
    metadata:
      labels:
        app: postgresql
    spec:
      securityContext:
        fsGroup: 999
      containers:
      - name: postgresql
        image: postgres:17-alpine
        env:
        - name: POSTGRES_DB
          value: "chat2sql"
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: postgresql-secret
              key: username
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgresql-secret
              key: password
        - name: PGDATA
          value: /var/lib/postgresql/data/pgdata
        ports:
        - containerPort: 5432
          name: postgresql
        volumeMounts:
        - name: postgresql-storage
          mountPath: /var/lib/postgresql/data
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
        livenessProbe:
          exec:
            command:
            - /bin/sh
            - -c
            - pg_isready -U $POSTGRES_USER -d $POSTGRES_DB
          initialDelaySeconds: 30
          periodSeconds: 10
  volumeClaimTemplates:
  - metadata:
      name: postgresql-storage
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: chat2sql-ssd
      resources:
        requests:
          storage: 100Gi
```

---

## 📦 Helm Chart开发

### 🎯 Chat2SQL Helm Chart结构

```bash
# Helm Chart目录结构
chat2sql-helm/
├── Chart.yaml
├── values.yaml
├── values-prod.yaml
├── values-staging.yaml
├── templates/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── configmap.yaml
│   ├── secret.yaml
│   ├── serviceaccount.yaml
│   ├── rbac.yaml
│   ├── hpa.yaml
│   ├── pdb.yaml
│   └── tests/
│       └── test-connection.yaml
├── charts/
└── crds/
```

### 📊 Chart.yaml配置

```yaml
# Chart.yaml
apiVersion: v2
name: chat2sql
description: Chat2SQL - AI-powered SQL query system
type: application
version: 1.0.0
appVersion: "1.0.0"
home: https://github.com/chat2sql/chat2sql
sources:
  - https://github.com/chat2sql/chat2sql
keywords:
  - ai
  - sql
  - database
  - langchain
maintainers:
  - name: Chat2SQL Team
    email: team@chat2sql.com
dependencies:
  - name: postgresql
    version: 12.x.x
    repository: https://charts.bitnami.com/bitnami
    condition: postgresql.enabled
  - name: redis
    version: 17.x.x
    repository: https://charts.bitnami.com/bitnami
    condition: redis.enabled
  - name: prometheus
    version: 15.x.x
    repository: https://prometheus-community.github.io/helm-charts
    condition: monitoring.prometheus.enabled
```

### 🔧 values.yaml核心配置

```yaml
# values.yaml
global:
  imageRegistry: ""
  imagePullSecrets: []
  storageClass: "chat2sql-ssd"

image:
  registry: ghcr.io
  repository: chat2sql/backend
  tag: "v1.0.0"
  pullPolicy: IfNotPresent

replicaCount: 3

service:
  type: ClusterIP
  port: 8080
  targetPort: 8080
  annotations: {}

ingress:
  enabled: true
  className: "istio"
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
  hosts:
    - host: chat2sql.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: chat2sql-tls
      hosts:
        - chat2sql.example.com

resources:
  limits:
    cpu: 1000m
    memory: 2Gi
  requests:
    cpu: 500m
    memory: 1Gi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 20
  targetCPUUtilizationPercentage: 70
  targetMemoryUtilizationPercentage: 80

postgresql:
  enabled: true
  auth:
    postgresPassword: "secure-password"
    database: "chat2sql"
  primary:
    persistence:
      enabled: true
      size: 100Gi
      storageClass: "chat2sql-ssd"

redis:
  enabled: true
  auth:
    enabled: true
    password: "secure-redis-password"
  master:
    persistence:
      enabled: true
      size: 20Gi
      storageClass: "chat2sql-ssd"

monitoring:
  prometheus:
    enabled: true
  grafana:
    enabled: true
  jaeger:
    enabled: true

security:
  podSecurityContext:
    runAsNonRoot: true
    runAsUser: 65534
    fsGroup: 65534
  securityContext:
    allowPrivilegeEscalation: false
    readOnlyRootFilesystem: true
    runAsNonRoot: true
    runAsUser: 65534
    capabilities:
      drop:
      - ALL

networkPolicy:
  enabled: true
  ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            name: istio-system
  egress:
    - to:
      - namespaceSelector:
          matchLabels:
            name: chat2sql
    - to: []
      ports:
      - protocol: TCP
        port: 53
      - protocol: UDP
        port: 53
```

### 🎨 Deployment模板

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ include "chat2sql.fullname" . }}
  labels:
    {{- include "chat2sql.labels" . | nindent 4 }}
spec:
  {{- if not .Values.autoscaling.enabled }}
  replicas: {{ .Values.replicaCount }}
  {{- end }}
  selector:
    matchLabels:
      {{- include "chat2sql.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      annotations:
        checksum/config: {{ include (print $.Template.BasePath "/configmap.yaml") . | sha256sum }}
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
      labels:
        {{- include "chat2sql.selectorLabels" . | nindent 8 }}
        version: {{ .Values.image.tag | quote }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      serviceAccountName: {{ include "chat2sql.serviceAccountName" . }}
      securityContext:
        {{- toYaml .Values.security.podSecurityContext | nindent 8 }}
      containers:
        - name: {{ .Chart.Name }}
          securityContext:
            {{- toYaml .Values.security.securityContext | nindent 12 }}
          image: "{{ .Values.image.registry }}/{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - name: http
              containerPort: 8080
              protocol: TCP
            - name: metrics
              containerPort: 9090
              protocol: TCP
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: {{ include "chat2sql.fullname" . }}-secret
                  key: database-url
            - name: REDIS_URL
              valueFrom:
                secretKeyRef:
                  name: {{ include "chat2sql.fullname" . }}-secret
                  key: redis-url
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 30
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
            timeoutSeconds: 3
            failureThreshold: 3
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
          volumeMounts:
            - name: config
              mountPath: /app/config
              readOnly: true
            - name: tmp
              mountPath: /tmp
      volumes:
        - name: config
          configMap:
            name: {{ include "chat2sql.fullname" . }}-config
        - name: tmp
          emptyDir: {}
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.affinity }}
      affinity:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with .Values.tolerations }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
```

---

## 🔄 GitOps配置

### 📦 ArgoCD配置

```yaml
# argocd-application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: chat2sql-prod
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    repoURL: https://github.com/chat2sql/chat2sql-helm
    targetRevision: main
    path: charts/chat2sql
    helm:
      valueFiles:
        - values-prod.yaml
      parameters:
        - name: image.tag
          value: "v1.0.0"
        - name: replicaCount
          value: "5"
  destination:
    server: https://kubernetes.default.svc
    namespace: chat2sql
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
      allowEmpty: false
    syncOptions:
      - CreateNamespace=true
      - PrunePropagationPolicy=foreground
      - PruneLast=true
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
---
# AppProject for Chat2SQL
apiVersion: argoproj.io/v1alpha1
kind: AppProject
metadata:
  name: chat2sql
  namespace: argocd
spec:
  description: Chat2SQL project
  sourceRepos:
  - 'https://github.com/chat2sql/*'
  destinations:
  - namespace: 'chat2sql*'
    server: https://kubernetes.default.svc
  clusterResourceWhitelist:
  - group: ''
    kind: Namespace
  - group: rbac.authorization.k8s.io
    kind: ClusterRole
  - group: rbac.authorization.k8s.io
    kind: ClusterRoleBinding
  namespaceResourceWhitelist:
  - group: ''
    kind: ConfigMap
  - group: ''
    kind: Secret
  - group: ''
    kind: Service
  - group: apps
    kind: Deployment
  - group: apps
    kind: StatefulSet
  roles:
  - name: chat2sql-dev
    description: Development team access
    policies:
    - p, proj:chat2sql:chat2sql-dev, applications, *, chat2sql/*, allow
    groups:
    - chat2sql:developers
```

### 🎯 多环境管理

```bash
# 多环境部署脚本
#!/bin/bash
set -e

ENVIRONMENT=${1:-staging}
IMAGE_TAG=${2:-latest}

echo "🚀 部署Chat2SQL到环境: $ENVIRONMENT"

case $ENVIRONMENT in
  "dev")
    NAMESPACE="chat2sql-dev"
    VALUES_FILE="values-dev.yaml"
    REPLICAS=1
    ;;
  "staging")
    NAMESPACE="chat2sql-staging"
    VALUES_FILE="values-staging.yaml"
    REPLICAS=2
    ;;
  "prod")
    NAMESPACE="chat2sql-prod"
    VALUES_FILE="values-prod.yaml"
    REPLICAS=5
    ;;
  *)
    echo "❌ 未知环境: $ENVIRONMENT"
    exit 1
    ;;
esac

# 更新Helm依赖
helm dependency update ./charts/chat2sql

# 部署应用
helm upgrade --install \
  chat2sql-$ENVIRONMENT \
  ./charts/chat2sql \
  --namespace $NAMESPACE \
  --create-namespace \
  --values ./charts/chat2sql/$VALUES_FILE \
  --set image.tag=$IMAGE_TAG \
  --set replicaCount=$REPLICAS \
  --wait \
  --timeout=10m

echo "✅ 部署完成: $ENVIRONMENT"

# 验证部署
kubectl get pods -n $NAMESPACE
kubectl get svc -n $NAMESPACE
kubectl get ingress -n $NAMESPACE

echo "🎉 Chat2SQL $ENVIRONMENT 环境就绪"
```

---

## 🧪 测试与验证

### 🔍 集群健康检查

```bash
#!/bin/bash
# cluster-health-check.sh

echo "🔍 Kubernetes集群健康检查"

# 检查节点状态
echo "📊 节点状态:"
kubectl get nodes -o wide

# 检查系统Pod状态
echo "🔧 系统组件状态:"
kubectl get pods -n kube-system

# 检查Istio状态
echo "🕸️ Istio状态:"
kubectl get pods -n istio-system
istioctl proxy-status

# 检查Chat2SQL应用状态
echo "🚀 Chat2SQL应用状态:"
kubectl get pods -n chat2sql
kubectl get svc -n chat2sql
kubectl get ingress -n chat2sql

# 资源使用情况
echo "📈 资源使用情况:"
kubectl top nodes
kubectl top pods -n chat2sql

# 事件检查
echo "📝 最近事件:"
kubectl get events -n chat2sql --sort-by='.lastTimestamp' | tail -10

echo "✅ 健康检查完成"
```

### 📊 性能测试

```yaml
# 性能测试Job
apiVersion: batch/v1
kind: Job
metadata:
  name: chat2sql-load-test
  namespace: chat2sql
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: load-test
        image: grafana/k6:latest
        command:
        - k6
        - run
        - --vus=100
        - --duration=10m
        - /scripts/load-test.js
        volumeMounts:
        - name: test-scripts
          mountPath: /scripts
        env:
        - name: TARGET_URL
          value: "https://chat2sql.example.com"
      volumes:
      - name: test-scripts
        configMap:
          name: load-test-scripts
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: load-test-scripts
  namespace: chat2sql
data:
  load-test.js: |
    import http from 'k6/http';
    import { check, sleep } from 'k6';

    export let options = {
      vus: 100,
      duration: '10m',
      thresholds: {
        http_req_duration: ['p(95)<2000'],
        http_req_failed: ['rate<0.1'],
      },
    };

    export default function() {
      // 测试API端点
      let response = http.post(`${__ENV.TARGET_URL}/api/query`, {
        query: "SELECT COUNT(*) FROM users WHERE active = true"
      }, {
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer test-token'
        },
      });

      check(response, {
        'status is 200': (r) => r.status === 200,
        'response time < 2s': (r) => r.timings.duration < 2000,
      });

      sleep(1);
    }
```

---

## 📚 最佳实践总结

### ✅ 推荐做法

1. **集群规划**
   - 使用多可用区部署提高可用性
   - 合理规划节点规格和数量
   - 预留足够的资源缓冲

2. **网络配置**
   - 选择合适的CNI插件(推荐Cilium)
   - 配置网络策略增强安全性
   - 使用服务网格管理服务间通信

3. **存储管理**
   - 使用高性能SSD存储
   - 配置存储类支持卷扩展
   - 定期备份持久化数据

4. **Helm最佳实践**
   - 使用语义化版本管理
   - 分环境配置values文件
   - 实现Helm Chart测试

### ❌ 避免的陷阱

1. **资源配置**
   - 避免资源请求设置过低导致调度失败
   - 避免资源限制设置过高造成浪费

2. **安全配置**
   - 不要在ConfigMap中存储敏感信息
   - 避免使用默认ServiceAccount

3. **网络策略**
   - 不要忽视网络策略配置
   - 避免过于宽松的安全策略

---

## 🔗 相关资源

- **Kubernetes官方文档**：https://kubernetes.io/docs/
- **Istio官方文档**：https://istio.io/latest/docs/
- **Helm官方文档**：https://helm.sh/docs/
- **ArgoCD官方文档**：https://argo-cd.readthedocs.io/

---

💡 **实施建议**：按照第1周的开发计划，先搭建基础集群，然后逐步配置服务网格、存储和GitOps，确保每个步骤都经过充分测试和验证。