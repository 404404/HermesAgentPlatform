package providers

import "context"

// RuntimeHostProvider is the infrastructure boundary for physical and virtual
// machines. The demo implementation deliberately reports deterministic mock
// information and never opens SSH or a Docker socket.
type RuntimeHostProvider interface {
	TestConnection(ctx context.Context, host RuntimeHost) (RuntimeHostInfo, error)
	GetHostInfo(ctx context.Context, host RuntimeHost) (RuntimeHostInfo, error)
	GetDockerInfo(ctx context.Context, host RuntimeHost) (DockerInfo, error)
	GetResources(ctx context.Context, host RuntimeHost) (HostResources, error)
	GetContainers(ctx context.Context, host RuntimeHost) ([]ContainerInfo, error)
	ProvisionRuntime(ctx context.Context, host RuntimeHost, runtimeID int64) error
	StopRuntime(ctx context.Context, host RuntimeHost, runtimeID int64) error
	RestartRuntime(ctx context.Context, host RuntimeHost, runtimeID int64) error
	DeleteRuntime(ctx context.Context, host RuntimeHost, runtimeID int64) error
}

type RuntimeHost struct{ ID int64; Name, Address string; SSHPort int }
type RuntimeHostInfo struct{ OS, Architecture string }
type DockerInfo struct{ Binary, Version, SocketPath string; SocketAccessible bool }
type HostResources struct{ CPU, Memory, Storage string; ContainerCount int }
type ContainerInfo struct{ ID, Name, Status string }

type MockRuntimeHostProvider struct{}

func (MockRuntimeHostProvider) TestConnection(ctx context.Context, host RuntimeHost) (RuntimeHostInfo, error) { return MockRuntimeHostProvider{}.GetHostInfo(ctx, host) }
func (MockRuntimeHostProvider) GetHostInfo(context.Context, RuntimeHost) (RuntimeHostInfo, error) { return RuntimeHostInfo{OS: "Mock Linux", Architecture: "amd64"}, nil }
func (MockRuntimeHostProvider) GetDockerInfo(context.Context, RuntimeHost) (DockerInfo, error) { return DockerInfo{Binary: "docker", Version: "mock-docker-27", SocketPath: "/var/run/docker.sock", SocketAccessible: true}, nil }
func (MockRuntimeHostProvider) GetResources(context.Context, RuntimeHost) (HostResources, error) { return HostResources{CPU: "8 CPU", Memory: "16 GB", Storage: "200 GB", ContainerCount: 0}, nil }
func (MockRuntimeHostProvider) GetContainers(context.Context, RuntimeHost) ([]ContainerInfo, error) { return []ContainerInfo{}, nil }
func (MockRuntimeHostProvider) ProvisionRuntime(context.Context, RuntimeHost, int64) error { return nil }
func (MockRuntimeHostProvider) StopRuntime(context.Context, RuntimeHost, int64) error { return nil }
func (MockRuntimeHostProvider) RestartRuntime(context.Context, RuntimeHost, int64) error { return nil }
func (MockRuntimeHostProvider) DeleteRuntime(context.Context, RuntimeHost, int64) error { return nil }
