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
	"strings"

	"github.com/goatkit/goatflow/pkg/plugin"
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
	Image      string               // Container image for the plugin
	Port       int                  // gRPC port
	MemoryMB   int                  // Memory limit from ResourcePolicy
	CPUMillis  int                  // CPU limit (millicores)
	Namespace  string               // K8s namespace (default: goatflow-plugins)
	Sidecars   []plugin.SidecarSpec // Sidecar containers declared in manifest
}

// GeneratePodManifest returns a Kubernetes pod YAML for a plugin.
// If the plugin declares sidecars, they are injected into the same pod
// so they share the pod network (sidecars reach each other via localhost).
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

	// Build sidecar container YAML blocks.
	sidecarsYAML := renderSidecars(spec.Sidecars)

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
          periodSeconds: 5%s
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
		sidecarsYAML,
		spec.PluginName, ns,
		spec.PluginName,
		spec.Port, spec.Port,
		spec.PluginName, ns,
		spec.PluginName,
		spec.Port,
	)
}

// renderSidecars generates YAML container blocks for plugin sidecars.
// Returns empty string if no sidecars declared.
func renderSidecars(sidecars []plugin.SidecarSpec) string {
	if len(sidecars) == 0 {
		return ""
	}

	var b strings.Builder
	for _, sc := range sidecars {
		mem := sc.MemoryMB
		if mem == 0 {
			mem = 128
		}
		cpu := sc.CPUMillis
		if cpu == 0 {
			cpu = 200
		}

		b.WriteString(fmt.Sprintf("\n      - name: %s\n        image: %s\n", sc.Name, sc.Image))

		// Ports.
		if len(sc.Ports) > 0 {
			b.WriteString("        ports:\n")
			for _, p := range sc.Ports {
				b.WriteString(fmt.Sprintf("        - containerPort: %s\n", p))
			}
		}

		// Resources.
		b.WriteString(fmt.Sprintf("        resources:\n          limits:\n            memory: %dMi\n            cpu: %dm\n          requests:\n            memory: %dMi\n            cpu: 50m\n",
			mem, cpu, mem/2))

		// Environment variables.
		if len(sc.Env) > 0 {
			b.WriteString("        env:\n")
			for k, v := range sc.Env {
				b.WriteString(fmt.Sprintf("        - name: %s\n          value: \"%s\"\n", k, v))
			}
		}

		// Security context for privileged containers.
		if sc.Privileged {
			b.WriteString("        securityContext:\n          privileged: true\n")
		}

		// Health check.
		if sc.Healthcheck != nil && len(sc.Healthcheck.Command) > 0 {
			interval := sc.Healthcheck.Interval
			if interval == "" {
				interval = "10s"
			}
			// Parse interval to seconds for K8s periodSeconds.
			periodSec := 10
			fmt.Sscanf(interval, "%ds", &periodSec)

			b.WriteString("        readinessProbe:\n          exec:\n            command:\n")
			for _, c := range sc.Healthcheck.Command {
				b.WriteString(fmt.Sprintf("            - \"%s\"\n", c))
			}
			b.WriteString(fmt.Sprintf("          initialDelaySeconds: 5\n          periodSeconds: %d\n", periodSec))
		}
	}

	return b.String()
}

// GenerateComposeFragment returns a Docker Compose service fragment for plugin sidecars.
// This can be merged into the project's docker-compose.override.yml.
func GenerateComposeFragment(pluginName string, sidecars []plugin.SidecarSpec) string {
	if len(sidecars) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Sidecars for %s plugin\nservices:\n", pluginName))

	for _, sc := range sidecars {
		b.WriteString(fmt.Sprintf("  %s-%s:\n    image: %s\n    container_name: %s\n    restart: unless-stopped\n",
			pluginName, sc.Name, sc.Image, sc.Name))

		if sc.Privileged {
			b.WriteString("    privileged: true\n")
		}

		if len(sc.Ports) > 0 {
			b.WriteString("    ports:\n")
			for _, p := range sc.Ports {
				b.WriteString(fmt.Sprintf("      - \"%s:%s\"\n", p, p))
			}
		}

		if len(sc.Env) > 0 {
			b.WriteString("    environment:\n")
			for k, v := range sc.Env {
				b.WriteString(fmt.Sprintf("      %s: \"%s\"\n", k, v))
			}
		}

		if len(sc.Volumes) > 0 {
			b.WriteString("    volumes:\n")
			for _, v := range sc.Volumes {
				b.WriteString(fmt.Sprintf("      - %s\n", v))
			}
		}

		if sc.Healthcheck != nil && len(sc.Healthcheck.Command) > 0 {
			b.WriteString("    healthcheck:\n      test: [\"CMD\"")
			for _, c := range sc.Healthcheck.Command {
				b.WriteString(fmt.Sprintf(", \"%s\"", c))
			}
			b.WriteString("]\n")
			interval := sc.Healthcheck.Interval
			if interval == "" {
				interval = "10s"
			}
			b.WriteString(fmt.Sprintf("      interval: %s\n      timeout: 3s\n      retries: 3\n", interval))
		}
	}

	return b.String()
}
