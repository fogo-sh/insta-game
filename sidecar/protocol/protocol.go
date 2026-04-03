package protocol

// ServerInfo holds the result of a game server status query.
// Fields not provided by a protocol are left as zero values.
type ServerInfo struct {
	Players  int
	Hostname string
	Map      string
}
