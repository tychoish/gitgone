package gitgone

import (
	"github.com/tychoish/gitgone/gitwrap"
)

type Repository interface {
	SetBranch(string) error
	Branch() string

	Clone(string, string) error
	CheckoutBranch(string, string) error
	Checkout(string) error
	CreateTrackingBranch(string, string, string) error
}

type RepositoryManager struct {
	Repository
}

func NewWrappedRepository(path string) *RepositoryManager {
	return &RepositoryManager{gitwrap.NewRepository(path)}
}

func (self *RepositoryManager) CloneMaster(remote string) error {
	return self.Clone(remote, "master")
}
