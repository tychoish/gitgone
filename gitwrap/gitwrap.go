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

	if self.exists && !self.bare && self.branches[branch] {
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

func (self *repository) checkGitCommand(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = self.path

	return cmd.Run()
}

func (self *repository) Clone(remote, branch string) (err error) {
	if _, err = os.Stat(self.path); err != nil && self.exists {
		return fmt.Errorf("could not clone %s (%s) into %s, because repository exists",
			remote, branch, self.path)
	}

	err = self.checkGitCommand("clone", remote, "--branch", branch, filepath.Dir(self.path))
	if err == nil {
		self.exists = true
	}

	return err
}

func (self *repository) CreateTrackingBranch(branch, remote, tracking string) error {
	if self.branches[branch] {
		return fmt.Errorf("branch '%s' exists, not creating a new branch.", branch)
	} else {
		err := self.checkGitCommand("branch", branch, strings.Join([]string{remote, tracking}, "/"))
		self.updateBranchTracking()

		return err
	}
}

func (self *repository) Checkout(ref string) error {
	if self.bare || !self.exists {
		return fmt.Errorf("cannot modify the working tree of this repository")
	}

	err := self.checkGitCommand("checkout", ref)
	if err != nil {
		self.state = states.UnresolvedOperation
	}

	return err
}

func (self *repository) CheckoutBranch(branch, starting string) error {
	if self.bare {
		return fmt.Errorf("cannot checkout new branch on a bare repository", branch)
	}

	if !self.exists {
		return fmt.Errorf("no repository exists at %s", self.path)
	}

	var err error

	if exists := self.branches[branch]; exists == true {
		self.branch = branch
		err = self.checkGitCommand("checkout", branch)
	} else {
		err = self.checkGitCommand("checkout", "-b", branch, starting)
		self.updateBranchTracking()
	}

	if err != nil {
		self.state = states.UnresolvedOperation
	}

	return err
}

func (self *repository) RemoveBranch(branch string, force bool) error {
	if exists := self.branches[branch]; exists == true {
		args := []string{"branch", "-d", branch}
		if force == true {
			args[1] = "-D"
		}

		return self.checkGitCommand(args...)
	} else {
		return fmt.Errorf("cannot remove branch %s, does not exist", branch)
	}
}

func (self *repository) Rebase(baseRef string) error {
	err := self.checkGitCommand("rebase", baseRef)
	if err != nil {
		self.state = states.UnresolvedOperation
	}
	return err
}

func (self *repository) Merge(baseRef string) error {
	return self.checkGitCommand("merge", baseRef)
}

func (self *repository) Reset(ref string, hard bool) error {
	var err error

	if hard {
		err = self.checkGitCommand("reset", "--hard", ref)
	} else {
		err = self.checkGitCommand("reset", ref)
	}

	if err != nil {
		self.state = states.UnresolvedOperation
	}

	return err

}

func (self *repository) Fetch(remote string) error {
	if remote == "all" {
		remote = "--all"
	}

	return self.checkGitCommand("fetch", remote)
}

func (self *repository) Pull(remote string, branch string, rebase bool) error {
	args := []string{"pull"}

	if rebase {
		args = append(args, "--rebase")
	}

	args = append(args, remote, branch)

	err := self.checkGitCommand(args...)
	if err != nil {
		self.state = states.UnresolvedOperation
	}
	return err
}

func (self *repository) CherryPick(commits ...string) error {
	for _, c := range commits {
		err := self.checkGitCommand(c)
		if err != nil {
			self.state = states.UnresolvedOperation
			return err
		}
	}

	return nil
}
