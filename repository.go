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

// The Repository interface provides an abstract, high-level set of
// operations that you can provide logically on a singe git
// repository. As an interface, this provides a way for client code to
// interfact with existing git repostories without needing to think
// about the underlying git implementation or interaction mode.
//
// Fundamentally, these operations are and should be realtively
// analogus to the common command line operations that a user migh
// perform on a git repository during normal development operations.
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

	// TOOD add commit staging, creation, push operations by
	// implementing the following methods:
	Stage(...stage) error
	StageAll() error
	Commit(string) error
	Ammend(string) error
}

// RepositoryManger embeds a Repository interface and provides acces
// to a number of common, operations in addition to the base methods
// provided by the interface.
type RepositoryManager struct {
	Repository
}

// Constructor for a RepositoryManager backed by an implementation
// that wrpas calles to the "git" binary. These operations are
// probably slower, on average, but will definitly behave in ways that
// proficent users of git may be more comfortable with. Use for
// repositories that you normally interact with using the "git"
// binary.
func NewWrappedRepository(path string) *RepositoryManager {
	return &RepositoryManager{gitwrap.NewRepository(path)}
}

// Constructor for a RepositoryManager backed by an implementation
// that uses the libgit2 implementation. libgit2 is an independent
// parallel implementation of git designed for library use. While
// operations are equivalent, to the "wrapped" equivalents, they may
// differ somewhat, particularyl for more proficient users. The direct
// operations are likely much more performant.
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
