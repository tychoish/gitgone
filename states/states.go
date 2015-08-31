package states

//go:generate stringer -type=RepositoryState
type RepositoryState int

const (
	New RepositoryState = iota
	Degraded
	UnresolvedOperation
	Detached
)
