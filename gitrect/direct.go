package gitrect

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"gopkg.in/libgit2/git2go.v23"

	"github.com/tychoish/gitgone/states"
	"github.com/tychoish/grip"
)

type repository struct {
	path   string
	exists bool
	repo   *git.Repository
	state  states.RepositoryState
	err    error
}

func NewRepository(path string) *repository {
	// canonical paths
	u, _ := user.Current()
	if strings.HasPrefix(path, "~") {
		path = filepath.Join(u.HomeDir, path[1:])
	}

	r := &repository{}

	resolvedPath, err := git.Discover(path, false, []string{path})
	if err == nil {
		r.exists = true
		r.path = resolvedPath
		r.repo, err = git.OpenRepository(r.path)
		if err != nil {
			r.state = states.Degraded
		}
	} else {
		r.exists = false
		r.path = path
		files, err := ioutil.ReadDir(r.path)
		if err == nil && len(files) > 0 {
			r.state = states.Degraded
			r.err = fmt.Errorf("files exists in repo path (%s)", r.path)
		} else {
			r.state = states.New
		}
	}

	return r
}

func (self *repository) Path() string {
	return self.path
}

func (self *repository) Branch() string {
	ref, err := self.repo.Head()
	if err != nil {
		self.state = states.Degraded
		return ""
	}

	name, err := ref.Branch().Name()
	if err != nil {
		self.state = states.Degraded
	}
	return name
}

func (self *repository) CreateBranch(name, starting string) error {
	if starting == "" {
		starting = "HEAD"
	}

	ref, err := self.repo.References.Dwim(starting)
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	commit, err := self.repo.LookupCommit(ref.Target())
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	_, err = self.repo.CreateBranch(name, commit, false)
	if err != nil {
		self.state = states.FailedOperation

	}

	return err
}

func (self *repository) BranchExists(name string) bool {
	_, err := self.repo.LookupBranch(name, git.BranchLocal)
	if err == nil {
		return true
	} else {
		return false
	}

}

func (self *repository) Clone(remote, branch string) (err error) {
	if _, err = os.Stat(self.path); err != nil && self.exists {
		return fmt.Errorf("could not clone %s (%s) into %s, because repository exists",
			remote, branch, self.path)
	}

	// clone a repository from a remote
	if err == nil {
		self.exists = true
	}

	return err
}

func (self *repository) Checkout(ref string) error {
	if self.repo.IsBare() || !self.exists {
		return fmt.Errorf("cannot modify the working tree of this repository")
	}

	tree, err := self.getTree(ref)
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	err = self.repo.CheckoutTree(tree, &git.CheckoutOpts{Strategy: git.CheckoutRecreateMissing})
	if err != nil {
		self.state = states.UnresolvedOperation
	}

	return err
}

func (self *repository) getTree(name string) (tree *git.Tree, err error) {
	ref, err := self.repo.References.Dwim(name)
	if err != nil {
		return
	}

	tree, err = self.repo.LookupTree(ref.Target())

	return
}

func (self *repository) IsBare() bool {
	return self.repo.IsBare()
}

func (self *repository) IsExists() bool {
	return self.exists
}

func (self *repository) RemoveBranch(branch string) error {
	var err error
	if self.BranchExists(branch) {
		branch, err := self.repo.LookupBranch(branch, git.BranchLocal)
		if err != nil {
			self.state = states.IncompleteOperation
			return err
		}
		err = branch.Delete()
		if err != nil {
			self.state = states.FailedOperation
			return err

		}
	} else {
		return fmt.Errorf("cannot remove branch %s, does not exist", branch)
	}

	return err
}

func (self *repository) Merge(baseRef string) error {
	mergeTree, err := self.getTree(baseRef)
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	headTree, err := self.getTree("HEAD")
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	mergeOpts, err := git.DefaultMergeOptions()
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	ancestorTree, err := self.getTree(self.Branch())
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	index, err := self.repo.MergeTrees(ancestorTree, headTree, mergeTree, &mergeOpts)
	if err != nil {
		self.state = states.UnresolvedOperation
		return err
	}

	err = self.repo.CheckoutIndex(index, &git.CheckoutOpts{Strategy: git.CheckoutRecreateMissing})
	if err != nil {
		self.state = states.UnresolvedOperation
		return err
	}

	return err
}

func (self *repository) Rebase(baseRef string) error {
	err := errors.New("rebases are not supported with the direct interface at this time.")
	grip.CatchEmergencyPanic(err)
	return err
}

func (self *repository) Reset(ref string, hard bool) error {
	// in more recent version of libgit2 there's a Reset method on
	// the repository object, and we should use this. In the mean
	// time the following implementation covers the common case.
	if hard {
		tree, err := self.getTree("HEAD")
		if err != nil {
			self.state = states.IncompleteOperation
			return err
		}

		err = self.repo.CheckoutTree(tree, &git.CheckoutOpts{Strategy: git.CheckoutUseTheirs})
		if err != nil {
			self.state = states.FailedOperation
		}
		return nil
	} else {
		index, err := self.repo.Index()
		if err != nil {
			self.state = states.IncompleteOperation
			return err
		}

		if index.Path() == "" {
			return nil
		} else {
			err = os.Remove(index.Path())
			if err != nil {
				self.state = states.FailedOperation
			}
			return nil
		}
	}
}

func (self *repository) Fetch(remote string) error {
	// TODO catche remotes in self
	// TODO make legit git signature constructor

	var remotes []*git.Remote

	remoteNames, err := self.repo.Remotes.List()
	if err != nil {
		return fmt.Errorf("no remotes defined")
	}

	for _, name := range remoteNames {
		r, err := self.repo.Remotes.Lookup(name)
		if err == nil {
			remotes = append(remotes, r)
		}
	}

	catcher := grip.NewCatcher()
	if remote == "all" {
		for _, remote := range remotes {
			catcher.Add(remote.Fetch([]string{}, &git.FetchOptions{}, ""))
		}
	}

	for _, remote := range remotes {
		if remote.Name() == remote.Name() {
			catcher.Add(remote.Fetch([]string{}, &git.FetchOptions{}, ""))
		}
	}

	return catcher.Resolve()
}

func (self *repository) Pull(remote string, branch string) error {
	err := self.Fetch(remote)
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	err = self.Merge(strings.Join([]string{remote, branch}, "/"))
	if err != nil {
		self.state = states.UnresolvedOperation

	}

	return err
}

func (self *repository) PullRebase(remote string, branch string) error {
	err := errors.New("rebases are not supported with the direct interface at this time.")
	grip.CatchEmergencyPanic(err)
	return err
}

func (self *repository) CherryPick(commits ...string) error {
	var resolvedCommits []*git.Commit
	for _, c := range commits {
		ref, err := self.repo.References.Dwim(c)
		if err != nil {
			self.state = states.IncompleteOperation
			return err
		}
		rCommit, err := self.repo.LookupCommit(ref.Target())
		if err != nil {
			self.state = states.IncompleteOperation
			return err
		}

		resolvedCommits = append(resolvedCommits, rCommit)
	}

	cpOpts, err := git.DefaultCherrypickOptions()
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}
	cpOpts.CheckoutOpts.Strategy = git.CheckoutRecreateMissing

	for _, rc := range resolvedCommits {
		err = self.repo.Cherrypick(rc, cpOpts)
		if err != nil {
			self.state = states.PartialOperation
			return err
		}

	}

	return nil
}

func (self *repository) Stage(fns ...string) error {
	index, err := self.repo.Index()
	if err != nil {
		return err
	}

	callback := func(path, matchedPathSpec string) int {
		grip.Debugf("adding file %s for %s to index", path, matchedPathSpec)
		return 0
	}

	return index.AddAll(fns, git.IndexAddCheckPathspec, callback)
}

func (self *repository) StageAllPath(path string) {
	index, err := self.repo.Index()
	if err != nil {
		return
	}

	callback := func(path, matchedPathSpec string) int {
		grip.Debugf("updating item %s (%s) in index.", path, matchedPathSpec)
		return 0
	}

	grip.CatchError(index.UpdateAll([]string{path}, callback))
}

func (self *repository) getCommitBasics() (signature *git.Signature, tree *git.Tree, err error) {
	signature, err = self.repo.DefaultSignature()
	if err != nil {
		return
	}

	index, err := self.repo.Index()
	if err != nil {
		return
	}

	ref, err := index.WriteTree()
	if err != nil {
		return
	}

	tree, err = self.repo.LookupTree(ref)

	return
}

func (self *repository) Commit(message string) error {
	sig, tree, err := self.getCommitBasics()
	if err != nil {
		self.state = states.UnresolvedOperation
		return err
	}

	commit, err := self.repo.CreateCommit("HEAD", sig, sig, message, tree)
	if err == nil {
		self.state = states.FailedOperation
		return err
	} else {
		grip.Debugf("created commit '%s' with message '%s' in repo '%s'",
			commit, message, self.path)
		return nil
	}
}

func (self *repository) CommitAll(message string) error {
	err := self.Stage(self.path)
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	err = self.Commit(message)
	if err != nil {
		self.state = states.FailedOperation
	}

	return err
}

func (self *repository) Amend(message string) error {
	signature, tree, err := self.getCommitBasics()
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	ref, err := self.repo.Head()
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	commit, err := self.repo.LookupCommit(ref.Target())
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	newCommit, err := commit.Amend("HEAD", commit.Author(), signature, message, tree)
	if err == nil {
		self.state = states.IncompleteOperation
		return err
	} else {
		grip.Debugf("amended commit '%s' to '%s' with message '%s' in repo '%s'",
			commit, newCommit, message, self.path)
		return nil
	}
}

func (self *repository) AmendAll(message string) error {
	err := self.Stage(self.path)

	if err != nil {
		self.state = states.IncompleteOperation
		return err
	} else {
	}

	err = self.Amend(message)
	if err != nil {
		self.state = states.FailedOperation
	}
	return err

}

func (self *repository) Push(remote, branch string) error {
	remoteRepo, err := self.repo.Remotes.Lookup(remote)
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	refspec := fmt.Sprintf("refs/heads/%s", branch)

	_, err = self.repo.References.Lookup(refspec)
	if err != nil {
		self.state = states.IncompleteOperation
		return err
	}

	err = remoteRepo.Push([]string{refspec}, nil)
	if err != nil {
		self.state = states.FailedOperation
	}

	return err
}

func (self *repository) CreateTag(name, sha, message string, force bool) error {
	oid, err := git.NewOid(sha)
	if err != nil {
		self.state = states.FailedOperation
		return err
	}

	commit, err := self.repo.LookupCommit(oid)
	if err != nil {
		self.state = states.IncompleteOperation
		return err

	}

	signature, err := self.repo.DefaultSignature()
	if err != nil {
		self.state = states.FailedOperation
		return err

	}

	if message == "" {
		oid, err = self.repo.Tags.CreateLightweight(name, commit, force)
	} else {
		oid, err = self.repo.Tags.Create(name, commit, signature, message)
	}

	grip.Debugf("created tag '%s' of commit '%s' with hash '%s' in repo '%s'",
		name, commit, self.path)

	return err
}

func (self *repository) DeleteTag(name string) error {
	tag, err := self.repo.References.Lookup(name)
	if err != nil {
		self.state = states.FailedOperation
		return err
	}

	err = tag.Delete()
	if err != nil {
		self.state = states.FailedOperation
		return err
	}

	return err
}

func (self *repository) IsTagged(name, sha string, lightweight bool) bool {
	oid, err := git.NewOid(sha)
	if err != nil {
		self.state = states.FailedOperation
		return false
	}

	ref, err := self.repo.References.Lookup(fmt.Sprintf("refs/tags/%s", name))
	if err != nil {
		return false
	}

	if ref.Target() == oid {
		if lightweight == true {
			return true
		} else {
			tag, err := self.repo.LookupTag(ref.Target())
			if err != nil {
				return false
			} else {
				if tag.Message() != "" {
					return false
				}
				return true
			}
		}
	}
	return false
}
