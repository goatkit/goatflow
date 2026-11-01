package landlock

// Landlock ABI versions each protection needs, and the first Linux release
// with that version.
const (
	// FilesABI (Linux 5.13): file access limits, and no access to the memory
	// or environment of processes outside the sandbox.
	FilesABI = 1
	// TCPABI (Linux 6.7): TCP bind and connect limits.
	TCPABI = 4
	// SignalsABI (Linux 6.12): no signals to processes outside the sandbox.
	SignalsABI = 6

	// FullKernel is the first Linux release with every protection used here.
	FullKernel = "6.12"
)

// Sandbox lists what a confined process may reach.
type Sandbox struct {
	ReadExec  []string // read and execute beneath these paths
	ReadWrite []string // full access beneath these paths
	DenyTCP   bool     // refuse TCP bind and connect
}
