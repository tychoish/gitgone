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

func (self *repository) BranchExists(name string) bool {
	self.updateBranchTracking()

	return self.branches[name]
}

func (self *repository) Path() string {
	return self.path
}

func (self *repository) Branch() string {
	self.updateBranchTracking()
	return self.branch
}

func (self *repository) getRef(name string) (string, error) {
	output, err := self.runGitCommand("rev-parse", "--verify", name)
	return output[0], err
}

func (self *repository) CreateBranch(name, starting string) error {
	if starting == "" {
		starting = "HEAD"
	}

	err := self.checkGitCommand("branch", name, starting)
	self.updateBranchTracking()
	return err
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

func (self *repository) IsBare() bool {
	return self.bare
}

func (self *repository) IsExists() bool {
	return self.exists
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

func (self *repository) RemoveBranch(branch string) error {
	if self.BranchExists(branch) {
		return self.checkGitCommand("branch", "-D", branch)
	} else {
		return fmt.Errorf("cannot remove branch %s, does not exist", branch)
	}
}

// Rebase() is not in the interface because git2go and gogit are both
// lack supprt for rebasing.
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

func (self *repository) Pull(remote string, branch string) error {
	err := self.checkGitCommand("pull", remote, branch)
	if err != nil {
		self.state = states.UnresolvedOperation
	}
	return err
}

func (self *repository) CherryPick(commits ...string) error {
	for _, c := range commits {
		err := self.checkGitCommand("cherry-pick", c)
		if err != nil {
			self.state = states.UnresolvedOperation
			return err
		}
	}

	return nil
}

func (self *repository) Stage(fns ...string) error {
	var missing []string

	for _, f := range fns {
		err := self.checkGitCommand("add", f)
		if err != nil {
			missing = append(missing, f)
		}
	}

	if len(missing) == 0 {
		return nil
	} else {
		return fmt.Errorf("error, could not add: %s", strings.Join(missing, ", "))
	}
}

func (self *repository) StageAllPath(path string) {
	oldCwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	if oldCwd != path {
		err = os.Chdir(path)
		if err != nil {
			panic(err)
		}
	}

	deletedFiles, _ := self.runGitCommand("ls-files -d")

	catcher := grip.NewCatcher()
	rmArgs := []string{"rm", "--quiet"}
	rmArgs = append(rmArgs, deletedFiles...)
	catcher.Add(self.checkGitCommand(rmArgs...))
	catcher.Add(self.checkGitCommand("add", "."))

	if oldCwd != path {
		err = os.Chdir(oldCwd)
		if err != nil {
			panic(err)
		}
	}
}

func (self *repository) Commit(message string) error {
	return self.checkGitCommand("commit", "--message", message)
}

func (self *repository) CommitAll(message string) error {
	return self.checkGitCommand("commit", "--all", "--message", message)
}

func (self *repository) Amend(message string) error {
	return self.checkGitCommand("commit", "--amend", "--message", message)
}

func (self *repository) AmendAll(message string) error {
	return self.checkGitCommand("commit", "--amend", "--all", "--message", message)
}

func (self *repository) Push(remote, branch string) error {
	return self.checkGitCommand("push", remote, branch)
}

func (self *repository) CreateTag(name, sha, message string, force bool) error {
	if sha == "" {
		sha = "HEAD"
	}

	if message == "" {
		if force {
			return self.checkGitCommand("tag", "--force", name, sha)
		} else {
			return self.checkGitCommand("tag", name, sha)
		}
	} else {
		if force {
			return self.checkGitCommand("tag", "--force", "--annotate",
				"--message", message, name, sha)
		} else {
			return self.checkGitCommand("tag", "--annotate", "--message", message, name, sha)
		}
	}
}

func (self *repository) DeleteTag(name string) error {
	return self.checkGitCommand("tag", "--delete", name)
}

func (self *repository) IsTagged(name, sha string, lightweight bool) bool {
	var err error

	if sha == "" || sha == "HEAD" {
		sha, err = self.getRef("HEAD")
		if err != nil {
			return false
		}
	}

	if lightweight {
		err = self.checkGitCommand("describe", "--tags", name)
		return true
	} else {
		err = self.checkGitCommand("describe", name)
	}

	if err != nil {
		grip.CatchError(err)
		return false
	}

	taggedCommit, err := self.runGitCommand("ref-list", "--max-count", "1", sha)
	if err != nil {
		grip.CatchError(err)
		return false
	}

	if taggedCommit[0] == sha {
		return true
	}
	return false
}
