# Kubernetes Deployment Guide

This guide covers deploying NebulaGC Server and Daemon on Kubernetes.

---

## Overview

Kubernetes provides orchestration, scaling, and self-healing for containerized applications. This guide includes:

- StatefulSet for server (database persistence)
- DaemonSet for daemon (one per node)
- Services, ConfigMaps, and Secrets
- High availability configuration
- Resource management and monitoring

---

## Prerequisites

- Kubernetes cluster 1.24+ (use `kubectl version`)
- `kubectl` configured to access your cluster
- Docker images built and pushed to registry
- Basic understanding of Kubernetes concepts

---

## Namespace

Create dedicated namespace:

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: nebulagc
  labels:
    app.kubernetes.io/name: nebulagc
    app.kubernetes.io/part-of: nebulagc-control-plane
```

Apply:

```bash
kubectl apply -f namespace.yaml
```

---

## Secrets

Create secret for HMAC key and sensitive configuration:

```bash
# Generate HMAC secret
HMAC_SECRET=$(openssl rand -hex 32)

# Create secret
kubectl create secret generic nebulagc-server-secret \
  --from-literal=hmac-secret="${HMAC_SECRET}" \
  --namespace=nebulagc
```

For daemon node tokens:

```bash
kubectl create secret generic nebulagc-daemon-secret \
  --from-literal=node-1-token="your-node-token-here" \
  --from-literal=node-2-token="another-node-token-here" \
  --namespace=nebulagc
```

Or use YAML:

```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: nebulagc-server-secret
  namespace: nebulagc
type: Opaque
stringData:
  hmac-secret: "your-generated-secret-here"
---
apiVersion: v1
kind: Secret
metadata:
  name: nebulagc-daemon-secret
  namespace: nebulagc
type: Opaque
stringData:
  node-token: "your-node-token-here"
```

---

## ConfigMap

Create ConfigMap for non-sensitive configuration:

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: nebulagc-server-config
  namespace: nebulagc
data:
  NEBULAGC_DB_PATH: "/var/lib/nebulagc-server/nebulagc.db"
  NEBULAGC_LISTEN_ADDR: "0.0.0.0:8080"
  NEBULAGC_LOG_LEVEL: "info"
  NEBULAGC_LOG_FORMAT: "json"
  NEBULAGC_LOG_SAMPLING: "true"
  NEBULAGC_RATELIMIT_AUTH_FAILURES_PER_MIN: "10"
  NEBULAGC_RATELIMIT_REQUESTS_PER_MIN: "100"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nebulagc-daemon-config
  namespace: nebulagc
data:
  config.yaml: |
    control_plane_urls:
      - "http://nebulagc-server-0.nebulagc-server:8080"
      - "http://nebulagc-server-1.nebulagc-server:8080"
      - "http://nebulagc-server-2.nebulagc-server:8080"
    
    clusters:
      - cluster_id: "cluster-1"
        node_id: "node-1"
        node_token: "${NODE_TOKEN}"
        config_dir: "/etc/nebula/cluster-1"
        provide_lighthouse: false
    
    poll_interval: 5s
    nebula_binary: "/usr/local/bin/nebula"
    state_dir: "/var/lib/nebulagc-daemon"
```

Apply:

```bash
kubectl apply -f configmap.yaml
```

---

## Server StatefulSet

StatefulSet ensures stable network identity and persistent storage:

```yaml
# server-statefulset.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: nebulagc-server
  namespace: nebulagc
  labels:
    app: nebulagc-server
    app.kubernetes.io/name: nebulagc-server
    app.kubernetes.io/component: control-plane
spec:
  serviceName: nebulagc-server
  replicas: 3
  selector:
    matchLabels:
      app: nebulagc-server
  template:
    metadata:
      labels:
        app: nebulagc-server
        app.kubernetes.io/name: nebulagc-server
        app.kubernetes.io/component: control-plane
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: nebulagc-server
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      
      containers:
      - name: server
        image: nebulagc-server:latest
        imagePullPolicy: Always
        
        ports:
        - name: http
          containerPort: 8080
          protocol: TCP
        
        env:
        # HA configuration based on pod ordinal
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NEBULAGC_HA_MODE
          value: "master"  # First pod is master, others are replicas (handled by init script)
        - name: NEBULAGC_HMAC_SECRET
          valueFrom:
            secretKeyRef:
              name: nebulagc-server-secret
              key: hmac-secret
        
        envFrom:
        - configMapRef:
            name: nebulagc-server-config
        
        volumeMounts:
        - name: data
          mountPath: /var/lib/nebulagc-server
        - name: init-script
          mountPath: /docker-entrypoint.d
        
        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3
        
        readinessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 2
        
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - ALL
      
      volumes:
      - name: init-script
        configMap:
          name: nebulagc-server-init
          defaultMode: 0755
      
      initContainers:
      - name: init-permissions
        image: alpine:3.19
        command: ['sh', '-c', 'chown -R 1000:1000 /var/lib/nebulagc-server']
        volumeMounts:
        - name: data
          mountPath: /var/lib/nebulagc-server
        securityContext:
          runAsUser: 0
  
  volumeClaimTemplates:
  - metadata:
      name: data
      labels:
        app: nebulagc-server
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
      # storageClassName: fast-ssd  # Optional: specify storage class
```

### HA Mode Init Script

Create ConfigMap for determining master/replica mode:

```yaml
# server-init-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: nebulagc-server-init
  namespace: nebulagc
data:
  00-set-ha-mode.sh: |
    #!/bin/sh
    # Determine HA mode based on pod ordinal
    POD_ORDINAL=$(echo $POD_NAME | rev | cut -d'-' -f1 | rev)
    
    if [ "$POD_ORDINAL" = "0" ]; then
      export NEBULAGC_HA_MODE="master"
      echo "Running as MASTER"
    else
      export NEBULAGC_HA_MODE="replica"
      export NEBULAGC_MASTER_URL="http://nebulagc-server-0.nebulagc-server:8080"
      echo "Running as REPLICA, master at $NEBULAGC_MASTER_URL"
    fi
```

---

## Server Service

### Headless Service (for StatefulSet)

```yaml
# server-service.yaml
apiVersion: v1
kind: Service
metadata:
  name: nebulagc-server
  namespace: nebulagc
  labels:
    app: nebulagc-server
spec:
  clusterIP: None  # Headless service for StatefulSet
  selector:
    app: nebulagc-server
  ports:
  - name: http
    port: 8080
    targetPort: 8080
    protocol: TCP
```

### Load Balancer Service (for external access)

```yaml
# server-service-lb.yaml
apiVersion: v1
kind: Service
metadata:
  name: nebulagc-server-lb
  namespace: nebulagc
  labels:
    app: nebulagc-server
  annotations:
    # Cloud provider specific annotations
    # AWS: service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    # GCP: cloud.google.com/load-balancer-type: "Internal"
spec:
  type: LoadBalancer
  selector:
    app: nebulagc-server
  ports:
  - name: http
    port: 8080
    targetPort: 8080
    protocol: TCP
  sessionAffinity: ClientIP  # Sticky sessions for HA
```

Apply:

```bash
kubectl apply -f server-service.yaml
kubectl apply -f server-service-lb.yaml
```

---

## Daemon DaemonSet

DaemonSet ensures one daemon pod runs on each node:

```yaml
# daemon-daemonset.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: nebulagc-daemon
  namespace: nebulagc
  labels:
    app: nebulagc-daemon
    app.kubernetes.io/name: nebulagc-daemon
    app.kubernetes.io/component: node-agent
spec:
  selector:
    matchLabels:
      app: nebulagc-daemon
  template:
    metadata:
      labels:
        app: nebulagc-daemon
        app.kubernetes.io/name: nebulagc-daemon
        app.kubernetes.io/component: node-agent
    spec:
      serviceAccountName: nebulagc-daemon
      hostNetwork: true  # Required for Nebula to manage host network
      
      # Run on all nodes including masters (optional)
      tolerations:
      - key: node-role.kubernetes.io/master
        operator: Exists
        effect: NoSchedule
      - key: node-role.kubernetes.io/control-plane
        operator: Exists
        effect: NoSchedule
      
      containers:
      - name: daemon
        image: nebulagc-daemon:latest
        imagePullPolicy: Always
        
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: NODE_TOKEN
          valueFrom:
            secretKeyRef:
              name: nebulagc-daemon-secret
              key: node-token
        
        volumeMounts:
        - name: config
          mountPath: /etc/nebulagc/daemon
        - name: state
          mountPath: /var/lib/nebulagc-daemon
        - name: nebula-config
          mountPath: /etc/nebula
        
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 256Mi
        
        securityContext:
          capabilities:
            add:
            - NET_ADMIN
            - NET_RAW
          privileged: false
      
      volumes:
      - name: config
        configMap:
          name: nebulagc-daemon-config
      - name: state
        hostPath:
          path: /var/lib/nebulagc-daemon
          type: DirectoryOrCreate
      - name: nebula-config
        hostPath:
          path: /etc/nebula
          type: DirectoryOrCreate
```

Apply:

```bash
kubectl apply -f daemon-daemonset.yaml
```

---

## Service Accounts and RBAC

### Server Service Account

```yaml
# server-rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: nebulagc-server
  namespace: nebulagc
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: nebulagc-server
  namespace: nebulagc
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
- apiGroups: [""]
  resources: ["secrets", "configmaps"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: nebulagc-server
  namespace: nebulagc
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: nebulagc-server
subjects:
- kind: ServiceAccount
  name: nebulagc-server
  namespace: nebulagc
```

### Daemon Service Account

```yaml
# daemon-rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: nebulagc-daemon
  namespace: nebulagc
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: nebulagc-daemon
rules:
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: nebulagc-daemon
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: nebulagc-daemon
subjects:
- kind: ServiceAccount
  name: nebulagc-daemon
  namespace: nebulagc
```

Apply:

```bash
kubectl apply -f server-rbac.yaml
kubectl apply -f daemon-rbac.yaml
```

---

## Ingress (Optional)

Expose server via Ingress controller:

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: nebulagc-server
  namespace: nebulagc
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - nebulagc.example.com
    secretName: nebulagc-tls
  rules:
  - host: nebulagc.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: nebulagc-server-lb
            port:
              number: 8080
```

---

## Persistent Volume

For cloud providers, use dynamic provisioning. For self-hosted:

```yaml
# pv.yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: nebulagc-server-pv-0
  labels:
    app: nebulagc-server
spec:
  capacity:
    storage: 10Gi
  accessModes:
  - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: local-storage
  local:
    path: /mnt/disks/ssd0
  nodeAffinity:
    required:
      nodeSelectorTerms:
      - matchExpressions:
        - key: kubernetes.io/hostname
          operator: In
          values:
          - node-1
```

---

## Deployment with Kustomize

Organize with Kustomize for environment-specific configs:

```
k8s/
├── base/
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── server-statefulset.yaml
│   ├── server-service.yaml
│   ├── daemon-daemonset.yaml
│   ├── configmap.yaml
│   └── rbac.yaml
├── overlays/
│   ├── dev/
│   │   ├── kustomization.yaml
│   │   └── resources.yaml
│   └── prod/
│       ├── kustomization.yaml
│       ├── resources.yaml
│       └── replicas.yaml
```

Base `kustomization.yaml`:

```yaml
# k8s/base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: nebulagc

resources:
- namespace.yaml
- server-rbac.yaml
- daemon-rbac.yaml
- configmap.yaml
- server-service.yaml
- server-service-lb.yaml
- server-statefulset.yaml
- daemon-daemonset.yaml

configMapGenerator:
- name: nebulagc-server-config
  literals:
  - NEBULAGC_LOG_LEVEL=info
  - NEBULAGC_LOG_FORMAT=json

secretGenerator:
- name: nebulagc-server-secret
  literals:
  - hmac-secret=changeme

images:
- name: nebulagc-server
  newTag: latest
- name: nebulagc-daemon
  newTag: latest
```

Production overlay:

```yaml
# k8s/overlays/prod/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
- ../../base

namespace: nebulagc-prod

replicas:
- name: nebulagc-server
  count: 5

patchesStrategicMerge:
- resources.yaml

configMapGenerator:
- name: nebulagc-server-config
  behavior: merge
  literals:
  - NEBULAGC_LOG_LEVEL=warn

images:
- name: nebulagc-server
  newTag: v1.0.0
- name: nebulagc-daemon
  newTag: v1.0.0
```

Deploy:

```bash
# Development
kubectl apply -k k8s/overlays/dev

# Production
kubectl apply -k k8s/overlays/prod
```

---

## Monitoring

### ServiceMonitor (Prometheus Operator)

```yaml
# servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: nebulagc-server
  namespace: nebulagc
  labels:
    app: nebulagc-server
spec:
  selector:
    matchLabels:
      app: nebulagc-server
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

### PodMonitor (for DaemonSet)

```yaml
# podmonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: nebulagc-daemon
  namespace: nebulagc
  labels:
    app: nebulagc-daemon
spec:
  selector:
    matchLabels:
      app: nebulagc-daemon
  podMetricsEndpoints:
  - port: http
    path: /metrics
    interval: 30s
```

---

## Autoscaling

### Horizontal Pod Autoscaler (HPA)

```yaml
# hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: nebulagc-server
  namespace: nebulagc
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: StatefulSet
    name: nebulagc-server
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

---

## Operations

### Deploy All Resources

```bash
# Create namespace
kubectl apply -f namespace.yaml

# Create secrets (with actual values)
kubectl create secret generic nebulagc-server-secret \
  --from-literal=hmac-secret="$(openssl rand -hex 32)" \
  --namespace=nebulagc

# Create ConfigMaps
kubectl apply -f configmap.yaml
kubectl apply -f server-init-configmap.yaml

# Create RBAC
kubectl apply -f server-rbac.yaml
kubectl apply -f daemon-rbac.yaml

# Deploy server
kubectl apply -f server-service.yaml
kubectl apply -f server-service-lb.yaml
kubectl apply -f server-statefulset.yaml

# Deploy daemon
kubectl apply -f daemon-daemonset.yaml

# Verify deployments
kubectl get all -n nebulagc
```

### Scale Server

```bash
# Scale up
kubectl scale statefulset nebulagc-server --replicas=5 -n nebulagc

# Scale down (careful with master!)
kubectl scale statefulset nebulagc-server --replicas=3 -n nebulagc
```

### Update Image

```bash
# Update server image
kubectl set image statefulset/nebulagc-server \
  server=nebulagc-server:v1.1.0 \
  -n nebulagc

# Update daemon image
kubectl set image daemonset/nebulagc-daemon \
  daemon=nebulagc-daemon:v1.1.0 \
  -n nebulagc

# Check rollout status
kubectl rollout status statefulset/nebulagc-server -n nebulagc
kubectl rollout status daemonset/nebulagc-daemon -n nebulagc
```

### View Logs

```bash
# Server logs
kubectl logs -f nebulagc-server-0 -n nebulagc

# All server logs
kubectl logs -l app=nebulagc-server -n nebulagc --tail=100

# Daemon logs from specific node
kubectl logs -l app=nebulagc-daemon -n nebulagc --field-selector spec.nodeName=node-1
```

### Access Shell

```bash
# Exec into server pod
kubectl exec -it nebulagc-server-0 -n nebulagc -- sh

# Exec into daemon pod
kubectl exec -it $(kubectl get pod -l app=nebulagc-daemon -n nebulagc -o jsonpath='{.items[0].metadata.name}') -n nebulagc -- sh
```

---

## Troubleshooting

### Pod Won't Start

```bash
# Describe pod
kubectl describe pod nebulagc-server-0 -n nebulagc

# Check events
kubectl get events -n nebulagc --sort-by='.lastTimestamp'

# Check logs
kubectl logs nebulagc-server-0 -n nebulagc
```

### PVC Not Binding

```bash
# Check PVC status
kubectl get pvc -n nebulagc

# Describe PVC
kubectl describe pvc data-nebulagc-server-0 -n nebulagc

# Check available PVs
kubectl get pv
```

### Network Issues

```bash
# Test service DNS
kubectl run -it --rm debug --image=alpine --restart=Never -- \
  nslookup nebulagc-server.nebulagc.svc.cluster.local

# Test connectivity
kubectl run -it --rm debug --image=alpine --restart=Never -- \
  wget -O- http://nebulagc-server:8080/health
```

### Resource Issues

```bash
# Check resource usage
kubectl top pods -n nebulagc
kubectl top nodes

# Describe pod for resource limits
kubectl describe pod nebulagc-server-0 -n nebulagc | grep -A5 Limits
```

---

## Best Practices

1. **Use StatefulSet for Server**: Provides stable identity and persistent storage
2. **Use DaemonSet for Daemon**: Ensures coverage on all nodes
3. **Configure Resource Limits**: Prevent resource exhaustion
4. **Use Secrets for Sensitive Data**: Never store secrets in ConfigMaps
5. **Enable RBAC**: Minimize permissions with service accounts
6. **Set Up Monitoring**: Use Prometheus ServiceMonitor
7. **Configure Health Checks**: Enable liveness and readiness probes
8. **Use Pod Disruption Budgets**: Maintain availability during updates
9. **Enable Network Policies**: Restrict traffic between pods
10. **Regular Backups**: Backup PVCs containing database

---

## Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [StatefulSet Concepts](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
- [DaemonSet Concepts](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/)
- [Kustomize Documentation](https://kustomize.io/)
- [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator)
