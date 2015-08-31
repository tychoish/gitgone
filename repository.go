package gitgone

//go:generate stringer -type=repositoryState
type repositoryState int

const (
	new repositoryState = iota
	degraded
)

type Repository interface {
	SetBranch(string) error
	Branch() string

	Clone(string, string) error
	CheckoutBranch(string, string) error
	Checkout(string) error
	CheckoutTrackingBranch(string, string) error
}

type RepositoryManager struct {
	Repository
}

func NewWrappedRepository(path string) *RepositoryManager {
	return &RepositoryManager{gitwrap.NewRepository()}
}

func (self *RepositoryManger) CloneMaster(remote string) error {
	return self.Clone(remote, "master")
}
