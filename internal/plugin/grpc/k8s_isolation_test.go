package grpc

import (
	"strings"
	"testing"

	"github.com/goatkit/goatflow/pkg/plugin"
)

func TestGeneratePodManifest_NoSidecars(t *testing.T) {
	yaml := GeneratePodManifest(K8sPodSpec{
		PluginName: "test-plugin",
		Image:      "goatkit/test-plugin:latest",
		Port:       50051,
		MemoryMB:   256,
		CPUMillis:  500,
	})

	if !strings.Contains(yaml, "name: goatflow-plugin-test-plugin") {
		t.Error("missing deployment name")
	}
	if !strings.Contains(yaml, "image: goatkit/test-plugin:latest") {
		t.Error("missing image")
	}
	if !strings.Contains(yaml, "containerPort: 50051") {
		t.Error("missing port")
	}
	if !strings.Contains(yaml, "NetworkPolicy") {
		t.Error("missing network policy")
	}
	// Should not contain sidecar references.
	if strings.Contains(yaml, "adb-server") {
		t.Error("unexpected sidecar in output")
	}
}

func TestGeneratePodManifest_WithSidecars(t *testing.T) {
	yaml := GeneratePodManifest(K8sPodSpec{
		PluginName: "goatkit-devices",
		Image:      "goatkit/goatkit-devices:latest",
		Port:       50051,
		MemoryMB:   256,
		Sidecars: []plugin.SidecarSpec{
			{
				Name:       "adb-server",
				Image:      "goatkit/adb-server:latest",
				Ports:      []string{"5037"},
				Privileged: true,
				MemoryMB:   128,
				Env:        map[string]string{"ADB_SERVER_HOST": "0.0.0.0"},
				Healthcheck: &plugin.SidecarHealth{
					Command:  []string{"adb", "devices"},
					Interval: "10s",
				},
			},
		},
	})

	if !strings.Contains(yaml, "name: adb-server") {
		t.Error("missing sidecar container name")
	}
	if !strings.Contains(yaml, "image: goatkit/adb-server:latest") {
		t.Error("missing sidecar image")
	}
	if !strings.Contains(yaml, "containerPort: 5037") {
		t.Error("missing sidecar port")
	}
	if !strings.Contains(yaml, "privileged: true") {
		t.Error("missing privileged flag")
	}
	if !strings.Contains(yaml, "ADB_SERVER_HOST") {
		t.Error("missing sidecar env var")
	}
	if !strings.Contains(yaml, `"adb"`) {
		t.Error("missing healthcheck command")
	}
}

func TestGenerateComposeFragment(t *testing.T) {
	yaml := GenerateComposeFragment("goatkit-devices", []plugin.SidecarSpec{
		{
			Name:       "adb-server",
			Image:      "goatkit/adb-server:latest",
			Ports:      []string{"5037"},
			Privileged: true,
			Env:        map[string]string{"ADB_SERVER_HOST": "0.0.0.0"},
			Volumes:    []string{"/dev/bus/usb:/dev/bus/usb"},
			Healthcheck: &plugin.SidecarHealth{
				Command:  []string{"adb", "devices"},
				Interval: "10s",
			},
		},
	})

	if !strings.Contains(yaml, "goatkit-devices-adb-server:") {
		t.Error("missing compose service name")
	}
	if !strings.Contains(yaml, "privileged: true") {
		t.Error("missing privileged flag")
	}
	if !strings.Contains(yaml, "/dev/bus/usb:/dev/bus/usb") {
		t.Error("missing volume mount")
	}
	if !strings.Contains(yaml, "ADB_SERVER_HOST") {
		t.Error("missing env var")
	}
}

func TestGenerateComposeFragment_Empty(t *testing.T) {
	yaml := GenerateComposeFragment("test", nil)
	if yaml != "" {
		t.Errorf("expected empty string for no sidecars, got: %s", yaml)
	}
}

func TestRenderSidecars_Empty(t *testing.T) {
	result := renderSidecars(nil)
	if result != "" {
		t.Errorf("expected empty string, got: %s", result)
	}
}
