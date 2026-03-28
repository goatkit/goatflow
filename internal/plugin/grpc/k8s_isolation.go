package grpc

// Kubernetes pod isolation for gRPC plugins.
//
// When GOATFLOW_PLUGIN_ISOLATION=k8s, gRPC plugins run as Kubernetes pods
// instead of local processes. Each plugin gets its own pod with:
//   - Resource limits (CPU, memory) from ResourcePolicy
//   - Network policy (only GoatFlow host can connect)
//   - Ephemeral storage (pod dies = state gone, except via HostAPI)
//   - Auto-restart on crash via Deployment controller
//
// This is an alternative to Linux namespace isolation (sandbox_linux.go)
// for environments where stronger isolation is required.

import (
	"fmt"
	"os"
)

// IsolationMode determines how gRPC plugins are sandboxed.
type IsolationMode string

const (
	// IsolationLocal runs plugins as local processes with Linux namespace isolation.
	IsolationLocal IsolationMode = "local"
	// IsolationK8s runs plugins as Kubernetes pods.
	IsolationK8s IsolationMode = "k8s"
)

// GetIsolationMode returns the configured plugin isolation mode.
func GetIsolationMode() IsolationMode {
	mode := os.Getenv("GOATFLOW_PLUGIN_ISOLATION")
	switch mode {
	case "k8s", "kubernetes":
		return IsolationK8s
	default:
		return IsolationLocal
	}
}

// K8sPodSpec generates a Kubernetes pod spec for a gRPC plugin.
// This is used by the plugin loader when isolation mode is "k8s".
type K8sPodSpec struct {
	PluginName string
	Image      string // Container image for the plugin
	Port       int    // gRPC port
	MemoryMB   int    // Memory limit from ResourcePolicy
	CPUMillis  int    // CPU limit (millicores)
	Namespace  string // K8s namespace (default: goatflow-plugins)
}

// GeneratePodManifest returns a Kubernetes pod YAML for a plugin.
func GeneratePodManifest(spec K8sPodSpec) string {
	ns := spec.Namespace
	if ns == "" {
		ns = "goatflow-plugins"
	}
	memLimit := spec.MemoryMB
	if memLimit == 0 {
		memLimit = 256
	}
	cpuLimit := spec.CPUMillis
	if cpuLimit == 0 {
		cpuLimit = 500
	}

	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: goatflow-plugin-%s
  namespace: %s
  labels:
    app: goatflow-plugin
    plugin: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      plugin: %s
  template:
    metadata:
      labels:
        app: goatflow-plugin
        plugin: %s
    spec:
      containers:
      - name: plugin
        image: %s
        ports:
        - containerPort: %d
          protocol: TCP
        resources:
          limits:
            memory: %dMi
            cpu: %dm
          requests:
            memory: %dMi
            cpu: 100m
        env:
        - name: GOATFLOW_HOST
          value: goatflow.goatflow.svc.cluster.local
        - name: PLUGIN_GRPC_PORT
          value: "%d"
        livenessProbe:
          tcpSocket:
            port: %d
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          tcpSocket:
            port: %d
          initialDelaySeconds: 3
          periodSeconds: 5
      restartPolicy: Always
---
apiVersion: v1
kind: Service
metadata:
  name: goatflow-plugin-%s
  namespace: %s
spec:
  selector:
    plugin: %s
  ports:
  - port: %d
    targetPort: %d
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: goatflow-plugin-%s-policy
  namespace: %s
spec:
  podSelector:
    matchLabels:
      plugin: %s
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          app: goatflow
    ports:
    - port: %d
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          app: goatflow
`,
		spec.PluginName, ns,
		spec.PluginName,
		spec.PluginName,
		spec.PluginName,
		spec.Image,
		spec.Port,
		memLimit, cpuLimit, memLimit/2,
		spec.Port,
		spec.Port,
		spec.Port,
		spec.PluginName, ns,
		spec.PluginName,
		spec.Port, spec.Port,
		spec.PluginName, ns,
		spec.PluginName,
		spec.Port,
	)
}
