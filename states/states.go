package states

//go:generate stringer -type=RepositoryState
type RepositoryState int

const (
	Unknown RepositoryState = iota
	Good
	New
	Detached
	Degraded
	UnresolvedOperation
	IncompleteOperation
	Salvaged
)
