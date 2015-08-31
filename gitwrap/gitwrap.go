package gitwrap

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/tychoish/gitgone/states"
	"github.com/tychoish/grip"
)

type repository struct {
	path   string
	branch string
	bare   bool
	exists bool
	state  states.RepositoryState

	branches map[string]bool
}

func NewRepository(path string) *repository {
	// canonical paths
	if strings.HasPrefix(path, "~") {
		u, _ := user.Current()
		path = filepath.Join(u.HomeDir, path[1:])
	}

	r := &repository{
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

		r.updateBranchTracking()
	} else {
		r.state = states.Degraded
	}

	return r
}

func (self *repository) updateBranchTracking() {
	branch, _ := self.runGitCommand("symbolic-ref", "--short", "HEAD")
	self.branch = strings.Join(branch, "\n")

	branches, _ := self.runGitCommand("branch", "--list", "--no-color")
	for _, b := range branches {
		b = strings.TrimLeft(b, " *")
		self.branches[b] = true
	}

	grip.Debug("updated branch tracking information.")
}

func (self *repository) Branch() string {
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
	if _, err := os.Stat(self.path); err != nil || self.exists {
		return fmt.Errorf("could not clone %s (%s) into %s, because repository exists",
			remote, branch, self.path)
	}

	_, err = self.runGitCommand("clone", remote, "--branch", branch, filepath.Dir(self.path))
	if err == nil {
		self.exists = true
	}

	return err
}

func (self *repository) CreateTrackingBranch(branch, remote, tracking string) error {
	if exists := self.branches[branch]; exists == true {
		return fmt.Errorf("branch '%s' exists, not creating a new branch.", branch)
	} else {
		_, err := self.runGitCommand("branch", branch, strings.Join([]string{remote, tracking}, "/"))
		self.updateBranchTracking()

		return err
	}
}

func (self *repository) Checkout(ref string) error {
	_, err := self.runGitCommand("checkout", ref)
	return err
}

func (self *repository) CheckoutBranch(branch, starting string) error {
	if exists := self.branches[branch]; exists == true {
		self.branch = branch
		_, err := self.runGitCommand("checkout", branch)
		return err
	} else {
		_, err := self.runGitCommand("checkout", "-b", branch, starting)

		self.updateBranchTracking()
		return err
	}
}
