package states

//go:generate stringer -type=repositoryState
type RepositoryState int

const (
	New RepositoryState = iota
	Degraded
)
