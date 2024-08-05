package defaults

const (
	// ContainerdNamespace is the name of the namespace to use with containerd.
	ContainerdNamespace = "vistara"

	// ContainerdSocket is the defaults path for the containerd socket.
	ContainerdSocket = "/var/lib/hypercore/containerd.sock"

	// Path to hac.toml
	HACFile = "hac.toml"

	// StateRootDir is the default directory to use for state information.
	StateRootDir = "/run/hypercore"

	// DataDirPerm is the permissions to use for data folders.
	DataDirPerm = 0o755

	// DataFilePerm is the permissions to use for data files.
	DataFilePerm = 0o644
)
