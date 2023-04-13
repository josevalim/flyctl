package appconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superfly/flyctl/api"
)

func TestToMachineConfig(t *testing.T) {
	cfg, err := LoadConfig("./testdata/tomachine.toml")
	require.NoError(t, err)

	want := &api.MachineConfig{
		Env: map[string]string{"FOO": "BAR", "PRIMARY_REGION": "mia"},
		Services: []api.MachineService{
			{
				Protocol:     "tcp",
				InternalPort: 8080,
				Ports: []api.MachinePort{
					{Port: api.Pointer(80), Handlers: []string{"http"}, ForceHttps: true},
					{Port: api.Pointer(443), Handlers: []string{"http", "tls"}, ForceHttps: false},
				},
			},
		},
		Metadata: map[string]string{"fly_platform_version": "v2", "fly_process_group": "app"},
		Metrics:  &api.MachineMetrics{Port: 9999, Path: "/metrics"},
		Statics:  []*api.Static{{GuestPath: "/guest/path", UrlPrefix: "/url/prefix"}},
		Mounts:   []api.MachineMount{{Name: "data", Path: "/data"}},
		Checks: map[string]api.MachineCheck{
			"listening": {Port: api.Pointer(8080), Type: api.Pointer("tcp")},
			"status": {
				Port:     api.Pointer(8080),
				Type:     api.Pointer("http"),
				Interval: mustParseDuration("10s"),
				Timeout:  mustParseDuration("1s"),
				HTTPPath: api.Pointer("/status"),
			},
		},
	}

	got, err := cfg.ToMachineConfig("", nil)
	assert.NoError(t, err)
	assert.Equal(t, want, got)

	// Update a machine config
	got, err = cfg.ToMachineConfig("", &api.MachineConfig{
		Guest:       &api.MachineGuest{CPUs: 3},
		Schedule:    "24/7",
		AutoDestroy: true,
		Restart:     api.MachineRestart{Policy: "poke"},
		DNS:         &api.DNSConfig{SkipRegistration: true},
		FlyProxy:    &api.MachineFlyProxy{AutostopMachine: api.Pointer(true)},
		Env:         map[string]string{"removed": "by-update"},
		Mounts:      []api.MachineMount{{Name: "removed", Path: "/by/update"}},
		Metadata:    map[string]string{"retain": "propagated"},
		Init:        api.MachineInit{Cmd: []string{"removed", "by", "update"}},
	})
	assert.NoError(t, err)
	assert.Equal(t, want.Env, got.Env)
	assert.Equal(t, want.Services, got.Services)
	assert.Equal(t, want.Checks, got.Checks)
	assert.Equal(t, &api.MachineGuest{CPUs: 3}, got.Guest)
	assert.Equal(t, "24/7", got.Schedule)
	assert.Equal(t, true, got.AutoDestroy)
	assert.Equal(t, api.MachineRestart{Policy: "poke"}, got.Restart)
	assert.Equal(t, &api.DNSConfig{SkipRegistration: true}, got.DNS)
	assert.Equal(t, &api.MachineFlyProxy{AutostopMachine: api.Pointer(true)}, got.FlyProxy)
	assert.Equal(t, "propagated", got.Metadata["retain"])
	assert.Empty(t, got.Init.Cmd)
}

func TestToMachineConfig_nullifyManagedFields(t *testing.T) {
	cfg := NewConfig()

	src := &api.MachineConfig{
		Env: map[string]string{"FOO": "BAR", "PRIMARY_REGION": "mia"},
		Services: []api.MachineService{
			{
				Protocol:     "tcp",
				InternalPort: 8080,
				Ports: []api.MachinePort{
					{Port: api.Pointer(80), Handlers: []string{"http"}, ForceHttps: true},
					{Port: api.Pointer(443), Handlers: []string{"http", "tls"}, ForceHttps: false},
				},
			},
		},
		Metrics: &api.MachineMetrics{Port: 9999, Path: "/metrics"},
		Statics: []*api.Static{{GuestPath: "/guest/path", UrlPrefix: "/url/prefix"}},
		Mounts:  []api.MachineMount{{Name: "data", Path: "/data"}},
		Checks: map[string]api.MachineCheck{
			"listening": {Port: api.Pointer(8080), Type: api.Pointer("tcp")},
			"status": {
				Port:     api.Pointer(8080),
				Type:     api.Pointer("http"),
				Interval: mustParseDuration("10s"),
				Timeout:  mustParseDuration("1s"),
				HTTPPath: api.Pointer("/status"),
			},
		},
	}

	got, err := cfg.ToMachineConfig("", src)
	require.NoError(t, err)
	assert.Empty(t, got.Env)
	assert.Empty(t, got.Metrics)
	assert.Empty(t, got.Services)
	assert.Empty(t, got.Checks)
	assert.Empty(t, got.Mounts)
	assert.Empty(t, got.Statics)
}

func TestToReleaseMachineConfig(t *testing.T) {
	cfg, err := LoadConfig("./testdata/tomachine.toml")
	require.NoError(t, err)

	want := &api.MachineConfig{
		Init:        api.MachineInit{Cmd: []string{"migrate-db"}},
		Env:         map[string]string{"FOO": "BAR", "PRIMARY_REGION": "mia", "RELEASE_COMMAND": "1"},
		Metadata:    map[string]string{"fly_platform_version": "v2", "fly_process_group": "fly_app_release_command"},
		AutoDestroy: true,
		Restart:     api.MachineRestart{Policy: api.MachineRestartPolicyNo},
		DNS:         &api.DNSConfig{SkipRegistration: true},
	}

	got, err := cfg.ToReleaseMachineConfig()
	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestToMachineConfig_multiProcessGroups(t *testing.T) {
	cfg, err := LoadConfig("./testdata/tomachine-processgroups.toml")
	require.NoError(t, err)

	testcases := []struct {
		name      string
		groupName string
		want      *api.MachineConfig
	}{
		{
			name:      "default empty process group",
			groupName: "app",
			want: &api.MachineConfig{
				Init: api.MachineInit{Cmd: []string{"run-nginx"}},
				Services: []api.MachineService{
					{
						Protocol:     "tcp",
						InternalPort: 8080,
						Ports: []api.MachinePort{
							{Port: api.Pointer(80), Handlers: []string{"http"}},
							{Port: api.Pointer(443), Handlers: []string{"http", "tls"}},
						},
					},
					{Protocol: "tcp", InternalPort: 1111},
				},
				Checks: map[string]api.MachineCheck{
					"listening": {Port: api.Pointer(8080), Type: api.Pointer("tcp")},
				},
			},
		},
		{
			name:      "vpn process group",
			groupName: "vpn",
			want: &api.MachineConfig{
				Init: api.MachineInit{Cmd: []string{"run-tailscale"}},
				Services: []api.MachineService{
					{Protocol: "udp", InternalPort: 9999},
					{Protocol: "tcp", InternalPort: 1111},
				},
			},
		},
		{
			name:      "foo process group",
			groupName: "foo",
			want: &api.MachineConfig{
				Init: api.MachineInit{Cmd: []string{"keep", "me", "alive"}},
				Services: []api.MachineService{
					{Protocol: "tcp", InternalPort: 1111},
				},
				Checks: map[string]api.MachineCheck{
					"listening": {Port: api.Pointer(8080), Type: api.Pointer("tcp")},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cfg.ToMachineConfig(tc.groupName, nil)
			require.NoError(t, err)
			// We only care about fields that change for different process groups
			assert.Equal(t, tc.groupName, got.Metadata["fly_process_group"])
			assert.Equal(t, tc.want.Init, got.Init)
			assert.Equal(t, tc.want.Services, got.Services)
			assert.Equal(t, tc.want.Checks, got.Checks)
		})
	}
}