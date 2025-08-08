# 🔄 自动化运维与DevOps指南

## 🎯 技术概述

自动化运维体系为Chat2SQL提供了完整的DevOps能力，通过CI/CD流水线、自动化部署、故障自愈等技术实现高效运维。本指南详细介绍P5阶段第4周的自动化运维与DevOps实现策略。

### ✨ 核心价值

| 功能特性 | 技术实现 | 业务价值 | 效率提升 |
|---------|---------|---------|---------| 
| **CI/CD流水线** | GitHub Actions + GitOps | 快速迭代部署 | 部署效率提升80% |
| **自动化备份** | CronJob + S3 + Velero | 数据安全保障 | 恢复时间减少90% |
| **故障自愈** | Kubernetes + 自定义控制器 | 系统稳定性 | 故障恢复时间减少85% |
| **性能优化** | HPA/VPA + 智能调度 | 资源利用优化 | 成本节省40% |

### 🎁 运维场景

- **持续集成**：代码提交自动触发构建、测试、部署流程
- **环境管理**：多环境自动化部署、配置管理、版本控制
- **监控告警**：全方位监控、智能告警、自动化响应
- **灾难恢复**：数据备份、故障切换、业务连续性保障

---

## 🚀 CI/CD流水线

### 📦 GitHub Actions工作流

```yaml
# .github/workflows/ci-cd.yml
name: Chat2SQL CI/CD Pipeline

on:
  push:
    branches: [main, develop]
    tags: ['v*']
  pull_request:
    branches: [main, develop]

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: chat2sql/backend
  HELM_CHART_PATH: ./charts/chat2sql

jobs:
  # 代码质量检查
  code-quality:
    runs-on: ubuntu-latest
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      
    - name: Setup Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
        
    - name: Cache Go modules
      uses: actions/cache@v3
      with:
        path: ~/go/pkg/mod
        key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
        
    - name: Run golangci-lint
      uses: golangci/golangci-lint-action@v3
      with:
        version: latest
        args: --timeout=5m
        
    - name: Run tests
      run: |
        go test -v -race -coverprofile=coverage.out ./...
        go tool cover -html=coverage.out -o coverage.html
        
    - name: Upload coverage
      uses: codecov/codecov-action@v3
      with:
        file: ./coverage.out
        
    - name: Security scan
      uses: securecodewarrior/github-action-add-sarif@v1
      with:
        sarif-file: gosec-report.sarif

  # 构建和推送镜像
  build-and-push:
    needs: code-quality
    runs-on: ubuntu-latest
    outputs:
      image-digest: ${{ steps.build.outputs.digest }}
      image-tag: ${{ steps.meta.outputs.tags }}
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      
    - name: Set up QEMU
      uses: docker/setup-qemu-action@v3
      
    - name: Set up Docker Buildx
      uses: docker/setup-buildx-action@v3
      
    - name: Login to Container Registry
      uses: docker/login-action@v3
      with:
        registry: ${{ env.REGISTRY }}
        username: ${{ github.actor }}
        password: ${{ secrets.GITHUB_TOKEN }}
        
    - name: Extract metadata
      id: meta
      uses: docker/metadata-action@v5
      with:
        images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
        tags: |
          type=ref,event=branch
          type=ref,event=pr
          type=semver,pattern={{version}}
          type=semver,pattern={{major}}.{{minor}}
          type=sha,prefix=commit-
          
    - name: Build and push
      id: build
      uses: docker/build-push-action@v5
      with:
        context: .
        platforms: linux/amd64,linux/arm64
        push: true
        tags: ${{ steps.meta.outputs.tags }}
        labels: ${{ steps.meta.outputs.labels }}
        cache-from: type=gha
        cache-to: type=gha,mode=max
        build-args: |
          VERSION=${{ github.ref_name }}
          COMMIT=${{ github.sha }}
          BUILD_TIME=${{ github.event.head_commit.timestamp }}

  # 容器安全扫描
  security-scan:
    needs: build-and-push
    runs-on: ubuntu-latest
    steps:
    - name: Run Trivy vulnerability scanner
      uses: aquasecurity/trivy-action@master
      with:
        image-ref: ${{ needs.build-and-push.outputs.image-tag }}
        format: 'sarif'
        output: 'trivy-results.sarif'
        
    - name: Upload Trivy scan results
      uses: github/codeql-action/upload-sarif@v2
      with:
        sarif_file: 'trivy-results.sarif'

  # Helm Chart验证
  helm-validation:
    runs-on: ubuntu-latest
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      
    - name: Set up Helm
      uses: azure/setup-helm@v3
      with:
        version: 'v3.15.0'
        
    - name: Lint Helm Chart
      run: |
        helm dependency update ${{ env.HELM_CHART_PATH }}
        helm lint ${{ env.HELM_CHART_PATH }}
        
    - name: Template Helm Chart
      run: |
        helm template test ${{ env.HELM_CHART_PATH }} \
          --values ${{ env.HELM_CHART_PATH }}/values-test.yaml > rendered.yaml
        kubectl --dry-run=server --validate=true apply -f rendered.yaml

  # 部署到开发环境
  deploy-dev:
    if: github.ref == 'refs/heads/develop'
    needs: [build-and-push, helm-validation, security-scan]
    runs-on: ubuntu-latest
    environment: development
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      
    - name: Configure kubectl
      uses: azure/k8s-set-context@v3
      with:
        method: kubeconfig
        kubeconfig: ${{ secrets.KUBE_CONFIG_DEV }}
        
    - name: Deploy to development
      run: |
        helm upgrade --install chat2sql-dev ${{ env.HELM_CHART_PATH }} \
          --namespace chat2sql-dev \
          --create-namespace \
          --values ${{ env.HELM_CHART_PATH }}/values-dev.yaml \
          --set image.tag=${{ github.sha }} \
          --wait --timeout=10m
          
    - name: Run smoke tests
      run: |
        kubectl wait --for=condition=ready pod -l app=chat2sql-backend -n chat2sql-dev --timeout=300s
        kubectl exec -n chat2sql-dev deployment/chat2sql-backend -- /app/healthcheck
        
    - name: Deploy status notification
      uses: 8398a7/action-slack@v3
      with:
        status: ${{ job.status }}
        channel: '#deployments'
        webhook_url: ${{ secrets.SLACK_WEBHOOK }}

  # 部署到生产环境
  deploy-prod:
    if: startsWith(github.ref, 'refs/tags/v')
    needs: [build-and-push, helm-validation, security-scan]
    runs-on: ubuntu-latest
    environment: production
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      
    - name: Configure kubectl
      uses: azure/k8s-set-context@v3
      with:
        method: kubeconfig
        kubeconfig: ${{ secrets.KUBE_CONFIG_PROD }}
        
    - name: Create backup
      run: |
        kubectl create job --from=cronjob/postgresql-backup backup-pre-deploy-$(date +%Y%m%d-%H%M%S) -n chat2sql
        
    - name: Deploy to production
      run: |
        helm upgrade --install chat2sql-prod ${{ env.HELM_CHART_PATH }} \
          --namespace chat2sql \
          --values ${{ env.HELM_CHART_PATH }}/values-prod.yaml \
          --set image.tag=${{ github.ref_name }} \
          --wait --timeout=15m
          
    - name: Run production tests
      run: |
        kubectl wait --for=condition=ready pod -l app=chat2sql-backend -n chat2sql --timeout=600s
        ./scripts/production-health-check.sh
        
    - name: Rollback on failure
      if: failure()
      run: |
        helm rollback chat2sql-prod -n chat2sql
        
    - name: Production deployment notification
      uses: 8398a7/action-slack@v3
      with:
        status: ${{ job.status }}
        channel: '#production-deployments'
        webhook_url: ${{ secrets.SLACK_WEBHOOK }}
```

### 🎯 GitOps工作流

```yaml
# .github/workflows/gitops.yml
name: GitOps Sync

on:
  push:
    branches: [main]
    paths: ['manifests/**', 'charts/**']

jobs:
  sync-gitops:
    runs-on: ubuntu-latest
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      
    - name: Setup ArgoCD CLI
      run: |
        curl -sSL https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64 -o argocd
        chmod +x argocd
        sudo mv argocd /usr/local/bin/
        
    - name: Login to ArgoCD
      run: |
        argocd login ${{ secrets.ARGOCD_SERVER }} \
          --username ${{ secrets.ARGOCD_USERNAME }} \
          --password ${{ secrets.ARGOCD_PASSWORD }} \
          --insecure
          
    - name: Sync applications
      run: |
        argocd app sync chat2sql-prod --prune
        argocd app wait chat2sql-prod --timeout 600
        
    - name: Check application health
      run: |
        argocd app get chat2sql-prod -o json | jq '.status.health.status'
        
    - name: Notification
      if: always()
      uses: 8398a7/action-slack@v3
      with:
        status: ${{ job.status }}
        text: "GitOps sync completed for Chat2SQL"
        webhook_url: ${{ secrets.SLACK_WEBHOOK }}
```

---

## 💾 自动化备份与恢复

### 📦 PostgreSQL自动化备份

```yaml
# postgresql-backup-cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: postgresql-backup
  namespace: chat2sql
spec:
  schedule: "0 2 * * *"  # 每天凌晨2点
  timeZone: "UTC"
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: 7
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      template:
        metadata:
          labels:
            app: postgresql-backup
        spec:
          restartPolicy: OnFailure
          serviceAccountName: backup-service-account
          containers:
          - name: backup
            image: postgres:17-alpine
            env:
            - name: PGHOST
              value: postgresql.chat2sql.svc.cluster.local
            - name: PGPORT
              value: "5432"
            - name: PGDATABASE
              value: chat2sql
            - name: PGUSER
              valueFrom:
                secretKeyRef:
                  name: postgresql-backup-secret
                  key: username
            - name: PGPASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgresql-backup-secret
                  key: password
            - name: AWS_ACCESS_KEY_ID
              valueFrom:
                secretKeyRef:
                  name: s3-backup-secret
                  key: access-key-id
            - name: AWS_SECRET_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: s3-backup-secret
                  key: secret-access-key
            - name: S3_BUCKET
              value: chat2sql-backups
            - name: S3_REGION
              value: us-west-2
            command:
            - /bin/bash
            - -c
            - |
              set -e
              
              BACKUP_DATE=$(date +%Y%m%d_%H%M%S)
              BACKUP_FILE="chat2sql_backup_${BACKUP_DATE}.sql"
              
              echo "开始备份数据库..."
              
              # 创建数据库备份
              pg_dump --verbose --no-owner --no-privileges \
                --format=custom \
                --file=/tmp/${BACKUP_FILE} \
                ${PGDATABASE}
              
              # 压缩备份文件
              gzip /tmp/${BACKUP_FILE}
              BACKUP_FILE="${BACKUP_FILE}.gz"
              
              # 安装AWS CLI
              apk add --no-cache aws-cli
              
              # 上传到S3
              echo "上传备份到S3..."
              aws s3 cp /tmp/${BACKUP_FILE} \
                s3://${S3_BUCKET}/postgresql/${BACKUP_FILE} \
                --region ${S3_REGION}
              
              # 验证上传
              aws s3 ls s3://${S3_BUCKET}/postgresql/${BACKUP_FILE} --region ${S3_REGION}
              
              echo "备份完成: ${BACKUP_FILE}"
              
              # 清理本地文件
              rm -f /tmp/${BACKUP_FILE}
              
              # 删除30天前的备份
              aws s3 ls s3://${S3_BUCKET}/postgresql/ --region ${S3_REGION} | \
                while read -r line; do
                  backup_date=$(echo $line | awk '{print $1" "$2}')
                  backup_file=$(echo $line | awk '{print $4}')
                  if [[ $(date -d "$backup_date" +%s) -lt $(date -d "30 days ago" +%s) ]]; then
                    echo "删除过期备份: $backup_file"
                    aws s3 rm s3://${S3_BUCKET}/postgresql/$backup_file --region ${S3_REGION}
                  fi
                done
            resources:
              requests:
                memory: 512Mi
                cpu: 250m
              limits:
                memory: 1Gi
                cpu: 500m
          - name: metrics-exporter
            image: prom/node-exporter:latest
            ports:
            - containerPort: 9100
            command: ["/bin/node_exporter", "--web.listen-address=:9100"]
```

### 🔄 Velero集群备份

```yaml
# velero-backup-schedule.yaml
apiVersion: velero.io/v1
kind: Schedule
metadata:
  name: chat2sql-daily-backup
  namespace: velero
spec:
  schedule: "0 3 * * *"  # 每天凌晨3点
  template:
    metadata:
      labels:
        backup-type: daily
    includedNamespaces:
    - chat2sql
    - monitoring
    - vault-system
    excludedResources:
    - events
    - events.events.k8s.io
    - backups.velero.io
    - restores.velero.io
    storageLocation: default
    volumeSnapshotLocations:
    - default
    ttl: 720h  # 30天
    hooks:
      resources:
      - name: postgresql-backup-hook
        includedNamespaces:
        - chat2sql
        includedResources:
        - pods
        labelSelector:
          matchLabels:
            app: postgresql
        pre:
        - exec:
            container: postgresql
            command:
            - /bin/bash
            - -c
            - |
              echo "开始数据库一致性检查..."
              psql -c "CHECKPOINT;" -d chat2sql
              psql -c "SELECT pg_start_backup('velero-backup', true);" -d chat2sql
        post:
        - exec:
            container: postgresql
            command:
            - /bin/bash
            - -c
            - |
              psql -c "SELECT pg_stop_backup();" -d chat2sql
              echo "数据库备份钩子完成"
---
# 周备份
apiVersion: velero.io/v1
kind: Schedule
metadata:
  name: chat2sql-weekly-backup
  namespace: velero
spec:
  schedule: "0 4 * * 0"  # 每周日凌晨4点
  template:
    metadata:
      labels:
        backup-type: weekly
    includedNamespaces:
    - chat2sql
    - monitoring
    - vault-system
    - istio-system
    - kube-system
    storageLocation: default
    ttl: 2160h  # 90天
```

### 📊 备份恢复脚本

```bash
#!/bin/bash
# restore-database.sh

set -e

BACKUP_DATE=${1:-$(date +%Y%m%d)}
S3_BUCKET="chat2sql-backups"
S3_REGION="us-west-2"
NAMESPACE="chat2sql"

echo "🔄 开始恢复Chat2SQL数据库备份..."

# 验证参数
if [[ -z "$BACKUP_DATE" ]]; then
    echo "❌ 错误: 请提供备份日期 (格式: YYYYMMDD)"
    exit 1
fi

# 查找备份文件
echo "📋 查找备份文件..."
BACKUP_FILE=$(aws s3 ls s3://${S3_BUCKET}/postgresql/ --region ${S3_REGION} | \
    grep "${BACKUP_DATE}" | tail -1 | awk '{print $4}')

if [[ -z "$BACKUP_FILE" ]]; then
    echo "❌ 错误: 找不到日期 ${BACKUP_DATE} 的备份文件"
    echo "可用备份:"
    aws s3 ls s3://${S3_BUCKET}/postgresql/ --region ${S3_REGION}
    exit 1
fi

echo "✅ 找到备份文件: ${BACKUP_FILE}"

# 确认恢复操作
read -p "⚠️  确认要恢复数据库到 ${BACKUP_DATE} 的状态吗? (y/N): " confirm
if [[ $confirm != [yY] ]]; then
    echo "❌ 恢复操作已取消"
    exit 0
fi

# 创建恢复作业
echo "🚀 创建恢复作业..."
cat <<EOF | kubectl apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: postgresql-restore-${BACKUP_DATE}
  namespace: ${NAMESPACE}
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: restore
        image: postgres:17-alpine
        env:
        - name: PGHOST
          value: postgresql.chat2sql.svc.cluster.local
        - name: PGPORT
          value: "5432"
        - name: PGDATABASE
          value: chat2sql
        - name: PGUSER
          valueFrom:
            secretKeyRef:
              name: postgresql-backup-secret
              key: username
        - name: PGPASSWORD
          valueFrom:
            secretKeyRef:
              name: postgresql-backup-secret
              key: password
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: s3-backup-secret
              key: access-key-id
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: s3-backup-secret
              key: secret-access-key
        command:
        - /bin/bash
        - -c
        - |
          set -e
          
          # 安装AWS CLI
          apk add --no-cache aws-cli
          
          # 下载备份文件
          echo "📥 下载备份文件..."
          aws s3 cp s3://${S3_BUCKET}/postgresql/${BACKUP_FILE} \
            /tmp/${BACKUP_FILE} --region ${S3_REGION}
          
          # 解压备份文件
          gunzip /tmp/${BACKUP_FILE}
          BACKUP_FILE=\${BACKUP_FILE%.gz}
          
          # 停止应用连接
          echo "⏸️  停止应用连接..."
          kubectl scale deployment chat2sql-backend --replicas=0 -n ${NAMESPACE} || true
          
          # 等待连接关闭
          sleep 30
          
          # 创建恢复前备份
          echo "💾 创建恢复前备份..."
          pg_dump --format=custom --file=/tmp/pre_restore_backup.sql \${PGDATABASE}
          
          # 恢复数据库
          echo "🔄 恢复数据库..."
          pg_restore --verbose --clean --if-exists \
            --no-owner --no-privileges \
            --dbname=\${PGDATABASE} /tmp/\${BACKUP_FILE}
          
          # 验证恢复
          echo "✅ 验证数据库恢复..."
          psql -c "SELECT COUNT(*) FROM users;" -d \${PGDATABASE}
          psql -c "SELECT version();" -d \${PGDATABASE}
          
          # 重新启动应用
          echo "🚀 重新启动应用..."
          kubectl scale deployment chat2sql-backend --replicas=3 -n ${NAMESPACE}
          
          echo "✅ 数据库恢复完成!"
        resources:
          requests:
            memory: 1Gi
            cpu: 500m
          limits:
            memory: 2Gi
            cpu: 1000m
EOF

# 等待作业完成
echo "⏳ 等待恢复作业完成..."
kubectl wait --for=condition=complete job/postgresql-restore-${BACKUP_DATE} \
  -n ${NAMESPACE} --timeout=1800s

# 检查作业状态
JOB_STATUS=$(kubectl get job postgresql-restore-${BACKUP_DATE} \
  -n ${NAMESPACE} -o jsonpath='{.status.conditions[0].type}')

if [[ "$JOB_STATUS" == "Complete" ]]; then
    echo "✅ 数据库恢复成功完成!"
    
    # 运行健康检查
    echo "🔍 运行应用健康检查..."
    kubectl wait --for=condition=ready pod -l app=chat2sql-backend \
      -n ${NAMESPACE} --timeout=300s
    
    kubectl exec -n ${NAMESPACE} deployment/chat2sql-backend -- \
      /app/healthcheck
    
    echo "🎉 恢复操作完全成功!"
else
    echo "❌ 恢复作业失败"
    kubectl logs job/postgresql-restore-${BACKUP_DATE} -n ${NAMESPACE}
    exit 1
fi

# 清理恢复作业
read -p "🧹 删除恢复作业? (Y/n): " cleanup
if [[ $cleanup != [nN] ]]; then
    kubectl delete job postgresql-restore-${BACKUP_DATE} -n ${NAMESPACE}
    echo "✅ 清理完成"
fi
```

---

## 🎯 故障自愈与自动化运维

### 📦 自定义控制器

```go
// 故障自愈控制器
package controller

import (
    "context"
    "fmt"
    "time"
    
    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/client-go/kubernetes"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

// 自愈控制器
type SelfHealingController struct {
    client.Client
    Scheme    *runtime.Scheme
    K8sClient kubernetes.Interface
    metrics   *PrometheusMetrics
}

// Pod状态检查和自愈
func (r *SelfHealingController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := ctrl.Log.WithValues("pod", req.NamespacedName)
    
    // 获取Pod
    var pod corev1.Pod
    if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // 检查Pod状态
    if r.isPodFailing(&pod) {
        log.Info("检测到Pod故障", "pod", pod.Name)
        
        if err := r.healPod(ctx, &pod); err != nil {
            log.Error(err, "Pod自愈失败")
            r.metrics.RecordHealingFailure(pod.Namespace, pod.Name)
            return ctrl.Result{RequeueAfter: time.Minute * 5}, err
        }
        
        r.metrics.RecordHealingSuccess(pod.Namespace, pod.Name)
        log.Info("Pod自愈成功", "pod", pod.Name)
    }
    
    return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
}

// 检查Pod是否故障
func (r *SelfHealingController) isPodFailing(pod *corev1.Pod) bool {
    // 检查重启次数
    for _, containerStatus := range pod.Status.ContainerStatuses {
        if containerStatus.RestartCount > 5 {
            return true
        }
        
        // 检查容器状态
        if containerStatus.State.Waiting != nil {
            reason := containerStatus.State.Waiting.Reason
            if reason == "CrashLoopBackOff" || reason == "ImagePullBackOff" {
                return true
            }
        }
    }
    
    // 检查Pod阶段
    if pod.Status.Phase == corev1.PodFailed {
        return true
    }
    
    // 检查资源使用情况
    if r.isResourceExhausted(pod) {
        return true
    }
    
    return false
}

// 执行Pod自愈
func (r *SelfHealingController) healPod(ctx context.Context, pod *corev1.Pod) error {
    log := ctrl.Log.WithValues("pod", pod.Name)
    
    // 获取Deployment
    deployment, err := r.getOwnerDeployment(ctx, pod)
    if err != nil {
        return fmt.Errorf("获取Deployment失败: %w", err)
    }
    
    if deployment == nil {
        log.Info("Pod不是Deployment管理，跳过自愈")
        return nil
    }
    
    // 检查是否需要扩容
    if r.shouldScaleUp(deployment) {
        if err := r.scaleUpDeployment(ctx, deployment); err != nil {
            return fmt.Errorf("扩容Deployment失败: %w", err)
        }
        log.Info("已扩容Deployment", "deployment", deployment.Name)
    }
    
    // 重启失败的Pod
    if err := r.restartPod(ctx, pod); err != nil {
        return fmt.Errorf("重启Pod失败: %w", err)
    }
    
    // 发送告警通知
    r.sendHealingNotification(pod, "Pod自愈操作已执行")
    
    return nil
}

// 重启Pod
func (r *SelfHealingController) restartPod(ctx context.Context, pod *corev1.Pod) error {
    // 删除Pod，让Deployment重新创建
    if err := r.Delete(ctx, pod); err != nil {
        return fmt.Errorf("删除Pod失败: %w", err)
    }
    
    // 等待新Pod启动
    time.Sleep(30 * time.Second)
    
    // 验证新Pod状态
    return r.waitForPodReady(ctx, pod.Namespace, pod.Labels)
}

// 扩容Deployment
func (r *SelfHealingController) scaleUpDeployment(ctx context.Context, deployment *appsv1.Deployment) error {
    if deployment.Spec.Replicas == nil {
        return fmt.Errorf("Deployment副本数为空")
    }
    
    currentReplicas := *deployment.Spec.Replicas
    newReplicas := currentReplicas + 1
    
    // 限制最大副本数
    if newReplicas > 10 {
        return fmt.Errorf("已达到最大副本数限制")
    }
    
    deployment.Spec.Replicas = &newReplicas
    
    if err := r.Update(ctx, deployment); err != nil {
        return fmt.Errorf("更新Deployment失败: %w", err)
    }
    
    return nil
}

// 检查是否需要扩容
func (r *SelfHealingController) shouldScaleUp(deployment *appsv1.Deployment) bool {
    if deployment.Spec.Replicas == nil {
        return false
    }
    
    currentReplicas := *deployment.Spec.Replicas
    readyReplicas := deployment.Status.ReadyReplicas
    
    // 如果就绪副本数少于期望副本数的50%，则扩容
    return float64(readyReplicas) < float64(currentReplicas)*0.5
}
```

### 🔧 自动化运维脚本

```bash
#!/bin/bash
# auto-ops.sh - 自动化运维脚本

set -e

NAMESPACE="chat2sql"
PROMETHEUS_URL="http://prometheus.monitoring.svc.cluster.local:9090"
SLACK_WEBHOOK_URL="https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK"

# 检查系统健康状态
check_system_health() {
    echo "🔍 检查系统健康状态..."
    
    # 检查Pod状态
    failed_pods=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Failed -o name)
    if [[ -n "$failed_pods" ]]; then
        echo "⚠️  发现失败的Pod:"
        echo "$failed_pods"
        
        # 自动清理失败的Pod
        echo "$failed_pods" | xargs kubectl delete -n $NAMESPACE
        send_notification "🧹 已清理失败的Pod: $failed_pods"
    fi
    
    # 检查资源使用情况
    check_resource_usage
    
    # 检查存储空间
    check_storage_usage
    
    # 检查网络连接
    check_network_connectivity
}

# 检查资源使用情况
check_resource_usage() {
    echo "📊 检查资源使用情况..."
    
    # 查询Prometheus指标
    cpu_usage=$(curl -s "${PROMETHEUS_URL}/api/v1/query?query=avg(rate(container_cpu_usage_seconds_total{namespace=\"$NAMESPACE\"}[5m]))" | \
        jq -r '.data.result[0].value[1]' 2>/dev/null || echo "0")
    
    memory_usage=$(curl -s "${PROMETHEUS_URL}/api/v1/query?query=avg(container_memory_working_set_bytes{namespace=\"$NAMESPACE\"})/1024/1024/1024" | \
        jq -r '.data.result[0].value[1]' 2>/dev/null || echo "0")
    
    # CPU使用率检查
    if (( $(echo "$cpu_usage > 0.8" | bc -l) )); then
        echo "⚠️  CPU使用率过高: ${cpu_usage}"
        auto_scale_up "high-cpu"
    fi
    
    # 内存使用率检查
    if (( $(echo "$memory_usage > 8" | bc -l) )); then
        echo "⚠️  内存使用率过高: ${memory_usage}GB"
        auto_scale_up "high-memory"
    fi
}

# 自动扩容
auto_scale_up() {
    local reason=$1
    echo "📈 触发自动扩容: $reason"
    
    deployments=$(kubectl get deployments -n $NAMESPACE -o name)
    for deployment in $deployments; do
        current_replicas=$(kubectl get $deployment -n $NAMESPACE -o jsonpath='{.spec.replicas}')
        max_replicas=10
        
        if [[ $current_replicas -lt $max_replicas ]]; then
            new_replicas=$((current_replicas + 1))
            kubectl scale $deployment --replicas=$new_replicas -n $NAMESPACE
            echo "✅ 扩容 $deployment: $current_replicas -> $new_replicas"
            
            send_notification "📈 自动扩容: $deployment ($reason)"
        fi
    done
}

# 检查存储使用情况
check_storage_usage() {
    echo "💾 检查存储使用情况..."
    
    # 检查PVC使用情况
    pvcs=$(kubectl get pvc -n $NAMESPACE -o json)
    echo "$pvcs" | jq -r '.items[] | select(.status.capacity.storage) | "\(.metadata.name) \(.status.capacity.storage)"' | \
    while read pvc_name capacity; do
        # 这里可以添加存储使用率检查逻辑
        echo "PVC: $pvc_name, 容量: $capacity"
    done
    
    # 检查节点存储空间
    kubectl top nodes | awk 'NR>1 {if($5+0 > 80) print "⚠️  节点存储使用率过高: " $1 " " $5}'
}

# 检查网络连接
check_network_connectivity() {
    echo "🌐 检查网络连接..."
    
    # 检查服务间连接
    kubectl exec -n $NAMESPACE deployment/chat2sql-backend -- \
        nc -zv postgresql.chat2sql.svc.cluster.local 5432 || \
        echo "❌ 数据库连接失败"
    
    kubectl exec -n $NAMESPACE deployment/chat2sql-backend -- \
        nc -zv redis.chat2sql.svc.cluster.local 6379 || \
        echo "❌ Redis连接失败"
}

# 自动清理资源
cleanup_resources() {
    echo "🧹 清理过期资源..."
    
    # 清理完成的Job
    kubectl delete jobs -n $NAMESPACE --field-selector=status.successful=1 \
        $(kubectl get jobs -n $NAMESPACE -o jsonpath='{.items[?(@.status.completionTime<"'$(date -d '7 days ago' -Iseconds)'")].metadata.name}') 2>/dev/null || true
    
    # 清理过期的Pod
    kubectl delete pods -n $NAMESPACE --field-selector=status.phase=Succeeded \
        $(kubectl get pods -n $NAMESPACE -o jsonpath='{.items[?(@.status.startTime<"'$(date -d '1 day ago' -Iseconds)'")].metadata.name}') 2>/dev/null || true
    
    # 清理过期的Secret（临时证书等）
    kubectl get secrets -n $NAMESPACE -o json | jq -r '.items[] | select(.metadata.annotations."cert-manager.io/certificate-name") | select(.metadata.creationTimestamp < "'$(date -d '30 days ago' -Iseconds)'") | .metadata.name' | \
    xargs -I {} kubectl delete secret {} -n $NAMESPACE 2>/dev/null || true
}

# 数据库维护
database_maintenance() {
    echo "🗄️  执行数据库维护..."
    
    # 数据库连接数检查
    active_connections=$(kubectl exec -n $NAMESPACE deployment/postgresql -- \
        psql -t -c "SELECT count(*) FROM pg_stat_activity WHERE state = 'active';" | tr -d ' ')
    
    if [[ $active_connections -gt 80 ]]; then
        echo "⚠️  数据库连接数过多: $active_connections"
        # 终止长时间运行的查询
        kubectl exec -n $NAMESPACE deployment/postgresql -- \
            psql -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'active' AND query_start < now() - interval '10 minutes';"
    fi
    
    # 数据库统计信息更新
    kubectl exec -n $NAMESPACE deployment/postgresql -- \
        psql -c "ANALYZE;" chat2sql
    
    # 清理过期的查询历史
    kubectl exec -n $NAMESPACE deployment/postgresql -- \
        psql -c "DELETE FROM query_history WHERE created_at < now() - interval '90 days';" chat2sql
}

# 发送通知
send_notification() {
    local message=$1
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    
    curl -X POST -H 'Content-type: application/json' \
        --data "{\"text\":\"[$timestamp] Chat2SQL自动运维: $message\"}" \
        $SLACK_WEBHOOK_URL || true
}

# 主函数
main() {
    echo "🚀 开始自动化运维检查 - $(date)"
    
    check_system_health
    cleanup_resources
    database_maintenance
    
    echo "✅ 自动化运维检查完成 - $(date)"
    send_notification "✅ 自动化运维检查完成"
}

# 如果脚本被直接执行
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
```

---

## 📊 性能优化与智能扩缩容

### 📦 KEDA自定义扩缩容

```yaml
# keda-scaledobject.yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: chat2sql-backend-scaler
  namespace: chat2sql
spec:
  scaleTargetRef:
    name: chat2sql-backend
  minReplicaCount: 2
  maxReplicaCount: 20
  cooldownPeriod: 300
  pollingInterval: 30
  triggers:
  # 基于CPU使用率
  - type: cpu
    metadata:
      type: Utilization
      value: "70"
  # 基于内存使用率
  - type: memory
    metadata:
      type: Utilization
      value: "80"
  # 基于HTTP请求队列长度
  - type: prometheus
    metadata:
      serverAddress: http://prometheus.monitoring.svc.cluster.local:9090
      metricName: http_requests_per_second
      threshold: "50"
      query: sum(rate(http_requests_total{job="chat2sql-backend"}[2m]))
  # 基于数据库连接数
  - type: prometheus
    metadata:
      serverAddress: http://prometheus.monitoring.svc.cluster.local:9090
      metricName: database_connections_active
      threshold: "80"
      query: sum(chat2sql_db_connections_active{pool="main"})
  # 基于AI推理队列长度
  - type: redis
    metadata:
      address: redis.chat2sql.svc.cluster.local:6379
      listName: ai_inference_queue
      listLength: "10"
      enableTLS: "false"
---
# 数据库扩缩容
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: postgresql-readonly-scaler
  namespace: chat2sql
spec:
  scaleTargetRef:
    name: postgresql-readonly
  minReplicaCount: 1
  maxReplicaCount: 5
  triggers:
  - type: prometheus
    metadata:
      serverAddress: http://prometheus.monitoring.svc.cluster.local:9090
      metricName: database_cpu_usage
      threshold: "60"
      query: avg(rate(container_cpu_usage_seconds_total{pod=~"postgresql-readonly-.*"}[5m])) * 100
```

### 🎯 VPA配置

```yaml
# vpa-recommender.yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: chat2sql-backend-vpa
  namespace: chat2sql
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: chat2sql-backend
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: chat2sql
      minAllowed:
        cpu: 100m
        memory: 256Mi
      maxAllowed:
        cpu: 2000m
        memory: 4Gi
      controlledResources: ["cpu", "memory"]
      controlledValues: RequestsAndLimits
---
# PostgreSQL VPA
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: postgresql-vpa
  namespace: chat2sql
spec:
  targetRef:
    apiVersion: apps/v1
    kind: StatefulSet
    name: postgresql
  updatePolicy:
    updateMode: "Off"  # 仅推荐，不自动更新
  resourcePolicy:
    containerPolicies:
    - containerName: postgresql
      minAllowed:
        cpu: 500m
        memory: 1Gi
      maxAllowed:
        cpu: 4000m
        memory: 16Gi
```

---

## 🧪 混沌工程与韧性测试

### 📦 Chaos Mesh实验

```yaml
# chaos-experiments.yaml
apiVersion: chaos-mesh.org/v1alpha1
kind: PodChaos
metadata:
  name: chat2sql-pod-failure
  namespace: chat2sql
spec:
  action: pod-kill
  mode: one
  duration: "30s"
  scheduler:
    cron: "0 */6 * * *"  # 每6小时执行一次
  selector:
    namespaces:
    - chat2sql
    labelSelectors:
      "app": "chat2sql-backend"
---
# 网络延迟实验
apiVersion: chaos-mesh.org/v1alpha1
kind: NetworkChaos
metadata:
  name: chat2sql-network-delay
  namespace: chat2sql
spec:
  action: delay
  mode: one
  duration: "5m"
  scheduler:
    cron: "0 2 * * 1"  # 每周一凌晨2点
  selector:
    namespaces:
    - chat2sql
    labelSelectors:
      "app": "chat2sql-backend"
  delay:
    latency: "100ms"
    correlation: "100"
    jitter: "10ms"
  direction: to
  target:
    mode: one
    selector:
      namespaces:
      - chat2sql
      labelSelectors:
        "app": "postgresql"
---
# 磁盘IO压力测试
apiVersion: chaos-mesh.org/v1alpha1
kind: StressChaos
metadata:
  name: chat2sql-disk-stress
  namespace: chat2sql
spec:
  mode: one
  duration: "10m"
  scheduler:
    cron: "0 3 * * 0"  # 每周日凌晨3点
  selector:
    namespaces:
    - chat2sql
    labelSelectors:
      "app": "postgresql"
  stressors:
    iomix:
      workers: 2
      size: "1GB"
```

### 🔍 韧性测试脚本

```bash
#!/bin/bash
# resilience-test.sh

set -e

NAMESPACE="chat2sql"
TEST_DURATION="300"  # 5分钟
CONCURRENT_USERS="50"

echo "🧪 开始韧性测试..."

# 运行负载测试
run_load_test() {
    echo "📈 启动负载测试..."
    
    kubectl run load-test --image=grafana/k6:latest --rm -i --restart=Never -- \
        run - <<EOF
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
    vus: $CONCURRENT_USERS,
    duration: '${TEST_DURATION}s',
    thresholds: {
        http_req_duration: ['p(95)<2000'],
        http_req_failed: ['rate<0.1'],
    },
};

export default function() {
    let response = http.post('http://chat2sql-backend.chat2sql.svc.cluster.local:8080/api/query', 
        JSON.stringify({
            query: "SELECT COUNT(*) FROM users WHERE active = true"
        }), 
        {
            headers: {
                'Content-Type': 'application/json',
                'Authorization': 'Bearer test-token'
            }
        }
    );
    
    check(response, {
        'status is 200': (r) => r.status === 200,
        'response time < 2s': (r) => r.timings.duration < 2000,
    });
    
    sleep(1);
}
EOF
}

# 故障注入测试
inject_failures() {
    echo "💥 注入故障进行测试..."
    
    # Pod故障
    kubectl apply -f - <<EOF
apiVersion: chaos-mesh.org/v1alpha1
kind: PodChaos
metadata:
  name: resilience-test-pod-kill
  namespace: $NAMESPACE
spec:
  action: pod-kill
  mode: fixed-percent
  value: "33"
  duration: "60s"
  selector:
    namespaces:
    - $NAMESPACE
    labelSelectors:
      "app": "chat2sql-backend"
EOF
    
    sleep 60
    
    # 网络故障
    kubectl apply -f - <<EOF
apiVersion: chaos-mesh.org/v1alpha1
kind: NetworkChaos
metadata:
  name: resilience-test-network-partition
  namespace: $NAMESPACE
spec:
  action: partition
  mode: one
  duration: "60s"
  selector:
    namespaces:
    - $NAMESPACE
    labelSelectors:
      "app": "chat2sql-backend"
  direction: both
  target:
    mode: one
    selector:
      namespaces:
      - $NAMESPACE
      labelSelectors:
        "app": "postgresql"
EOF
    
    sleep 60
}

# 监控和收集指标
monitor_metrics() {
    echo "📊 收集韧性测试指标..."
    
    # 错误率
    error_rate=$(kubectl exec -n monitoring deployment/prometheus -- \
        promtool query instant 'rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m])' | \
        grep -o '[0-9.]*' | tail -1)
    
    # 平均响应时间
    avg_response_time=$(kubectl exec -n monitoring deployment/prometheus -- \
        promtool query instant 'histogram_quantile(0.5, rate(http_request_duration_seconds_bucket[5m]))' | \
        grep -o '[0-9.]*' | tail -1)
    
    # 恢复时间
    recovery_time=$(kubectl exec -n monitoring deployment/prometheus -- \
        promtool query instant 'time() - on() chat2sql_last_failure_timestamp' | \
        grep -o '[0-9.]*' | tail -1)
    
    echo "📈 韧性测试结果:"
    echo "   错误率: ${error_rate}%"
    echo "   平均响应时间: ${avg_response_time}s"
    echo "   恢复时间: ${recovery_time}s"
}

# 清理测试资源
cleanup() {
    echo "🧹 清理测试资源..."
    
    kubectl delete podchaos resilience-test-pod-kill -n $NAMESPACE --ignore-not-found
    kubectl delete networkchaos resilience-test-network-partition -n $NAMESPACE --ignore-not-found
    
    echo "✅ 清理完成"
}

# 主函数
main() {
    trap cleanup EXIT
    
    echo "🚀 开始韧性测试 - $(date)"
    
    # 并行运行负载测试和故障注入
    run_load_test &
    LOAD_TEST_PID=$!
    
    sleep 60  # 让负载测试先运行1分钟
    inject_failures
    
    wait $LOAD_TEST_PID
    
    monitor_metrics
    
    echo "✅ 韧性测试完成 - $(date)"
}

main "$@"
```

---

## 📚 最佳实践总结

### ✅ 推荐做法

1. **CI/CD流水线**
   - 自动化测试覆盖率>80%
   - 多环境部署策略
   - 回滚机制完善

2. **备份策略**
   - 自动化定期备份
   - 跨地域备份存储
   - 定期恢复演练

3. **监控告警**
   - 业务指标优先
   - 分级告警机制
   - 自动化响应

4. **故障处理**
   - 预定义处理流程
   - 自动化故障恢复
   - 故障复盘机制

### ❌ 避免的陷阱

1. **过度自动化**
   - 避免自动化关键操作无人审核
   - 保留手动干预能力

2. **备份疏漏**
   - 不要忽视备份验证
   - 避免单点备份失败

3. **告警过载**
   - 避免告警风暴
   - 合理设置告警阈值

---

## 🔗 相关资源

- **Kubernetes官方运维指南**：https://kubernetes.io/docs/tasks/
- **Prometheus监控最佳实践**：https://prometheus.io/docs/practices/
- **Chaos Engineering原理**：https://principlesofchaos.org/
- **GitOps工作流指南**：https://www.gitops.tech/

---

💡 **实施建议**：按照第4周的开发计划，先建立CI/CD流水线，然后实现自动化备份、故障自愈机制，最后进行韧性测试验证，确保运维自动化程度达到90%以上。