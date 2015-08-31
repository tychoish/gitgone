// gitgone is a high level library for interacting with git
// repositories. It provides two implementations of the Repository
// interface, backed both by calls to the external git binary
// ("wrapped"), a second using libgit2 and git2gone. In most cases the
// libgit implementation is preferable: for speed and clarity, but the
// the "wrapped" implemenation is useful for validations and use
// platforms or deployments that lack easy access to libgit2.
//
// In most cases, use the RepositoryManager type, which wraps the
// Repository interface some additonal convinence helpers.
package gitgone

import (
	"github.com/tychoish/gitgone/gitwrap"
)

type Repository interface {
	Branch() string
	SetBranch(string) error

	Clone(string, string) error
	Checkout(string) error

	CheckoutBranch(string, string) error
	CreateTrackingBranch(string, string, string) error
	RemoveBranch(string, bool) error

	Rebase(string) error
	Merge(string) error
	Reset(string, bool) error
	CherryPick(...string) error

	Fetch(string) error
	Pull(string, string, bool) error
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

func (self *RepositoryManager) RemoveBranchSafe(branch string) error {
	return self.RemoveBranch(branch, false)
}

func (self *RepositoryManager) RemoveBranchForce(branch string) error {
	return self.RemoveBranch(branch, true)
}

func (self *RepositoryManager) ResetHeadHard() error {
	return self.Reset("HEAD", true)
}

func (self *RepositoryManager) ResetHead() error {
	return self.Reset("HEAD", false)
}
