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
	"fmt"
	"strings"

	"github.com/tychoish/gitgone/gitrect"
	"github.com/tychoish/gitgone/gitwrap"
)

type Repository interface {
	Path() string
	Branch() string
	BranchExists(string) bool
	IsBare() bool
	IsExists() bool

	Clone(string, string) error
	Checkout(string) error

	CreateBranch(string, string) error
	RemoveBranch(string) error

	Merge(string) error
	Reset(string, bool) error
	CherryPick(...string) error

	Fetch(string) error
	Pull(string, string) error
}

type RepositoryManager struct {
	Repository
}

func NewWrappedRepository(path string) *RepositoryManager {
	return &RepositoryManager{gitwrap.NewRepository(path)}
}

func NewDirectRepository(path string) *RepositoryManager {
	return &RepositoryManager{gitrect.NewRepository(path)}
}

func (self *RepositoryManager) CloneMaster(remote string) error {
	return self.Clone(remote, "master")
}

func (self *RepositoryManager) ResetHeadHard() error {
	return self.Reset("HEAD", true)
}

func (self *RepositoryManager) ResetHead() error {
	return self.Reset("HEAD", false)
}

func (self *RepositoryManager) CheckoutBranch(branch, starting string) error {
	if self.IsBare() {
		return fmt.Errorf("cannot checkout new branch on a bare repository", branch)
	}
	if !self.IsExists() {
		return fmt.Errorf("no repository exists at %s", self.Path())
	}

	if !self.BranchExists(branch) {
		err := self.CreateBranch(branch, starting)
		if err != nil {
			return err
		}
	}

	return self.Checkout(branch)
}

func (self *RepositoryManager) CreateTrackingBranch(branch, remote, tracking string) error {
	if self.BranchExists(branch) {
		return fmt.Errorf("branch '%s' exists, not creating a new branch.", branch)
	} else {
		return self.CheckoutBranch(branch, strings.Join([]string{remote, tracking}, "/"))
	}
}
