package gitwrap

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/tychoish/grip"
)

type repository struct {
	path   string
	branch string
	bare   bool
	exists bool
	state  repositoryState

	branches map[string]bool
}

func NewRepository(path string) *repository {
	// canonical paths
	if strings.HasPrefix(path, "~") {
		u, _ := user.Current()
		path = filepath.Join(u.HomeDir, path[1:])
	}

	r := &Repository{
		path:     path,
		bare:     false,
		branches: make(map[string]bool),
	}

	output, err := r.runGitCommand("rev-parse", "--is-bare-repository")

	if err == nil {
		r.exists = true
		if output[0] == "true" {
			r.bare = true
		}

		self.updateBranchTracking()
	} else {
		r.state = degraded
	}

	return r
}

func (self *repository) updateBranchTracking() {
	branch, _ := r.runGitCommand("symbolic-ref", "--short", "HEAD")
	r.branch = strings.Join(branch, "\n")

	branches, _ := r.runGitCommand("branch", "--list", "--no-color")
	for _, b := range strings.Split(branches, "\n") {
		self.branches[b] = true
	}

	grip.Debug("updated branch tracking information.")
}

func (self *repository) Branch() {
	self.updateBranchTracking()
	return self.branch
}

func (self *repository) SetBranch(branch string) (err error) {
	self.updateBranchTracking()

	if branch != self.branch {
		self.branch = branch
	}

	if exists := self.branches[branch]; exists == true {
		err = self.Checkout(branch)
	}

	return
}

func (self *repository) runGitCommand(args ...string) ([]string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = self.path

	output, err := cmd.CombinedOutput()

	return strings.Split(strings.Trim(string(output), " \t\n\r"), "\n"), err
}

func (self *repository) Clone(remote, branch string) (err error) {
	if self.exists || _, err = os.Stat(self.path); err != nil {
		return fmt.Errorf("could not clone %s (%s) into %s, because repository exists",
			remote, branch, self.path)

	}
	_, err := self.runGitCommand("clone", remote, "--branch", branch, filepath.Dir(self.path))
	if err == nil {
		self.exists = true
	}

	return err
}

func (self *repository) CreateTrackingBranch(branch, remote, tracking string) error {
	if exists := self.branches[branch]; exists == true {
		return fmt.Errorf("branch '%s' exists, not creating a new branch.", branch)
	} else {
		err := self.runGitCommand("branch", branch, strings.Join([]string{remote, tracking}, "/"))
		self.updateBranchTracking()

		return
	}
}

func (self *repository) Checkout(ref string) error {
	return self.runGitCommand("checkout", ref)
}

func (self *repository) CheckoutBranch(branch, starting string) error {
	if exists := self.branches[branch]; exists == true {
		self.branch = branch
		return self.runGitCommand("checkout", branch)
	} else {
		err := self.runGitCommand("checkout", "-b", branch, starting)

		self.updateBranchTracking()
		return err
	}
}
