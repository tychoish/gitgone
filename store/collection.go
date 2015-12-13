package store

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/tychoish/grip"
	"gopkg.in/libgit2/git2go.v23"
)

type Collection struct {
	name   string
	db     *Database
	tree   *git.Tree
	commit *git.Commit
	sync.RWMutex
}

func (db *Database) Collection(name string) *Collection {
	coll, ok := db.collections[name]
	if ok {
		db.RLock()
		defer db.RUnlock()

		coll.Lock()
		coll.setCommit()
		coll.Unlock()

		return coll
	} else {
		coll := db.createCollectionUnsafe(name)

		db.Lock()
		defer db.Unlock()
		db.collections[name] = coll

		return coll
	}
}

func (db *Database) createCollectionUnsafe(name string) *Collection {
	coll := &Collection{
		name: name,
		db:   db,
	}
	coll.Lock()
	coll.resetUnsafe()
	coll.Unlock()

	return coll
}

func (c *Collection) Get(key string) (out []byte, err error) {
	c.RLock()
	defer c.RUnlock()

	if c.tree == nil {
		return out, os.ErrNotExist
	}
	key = treePath(key)
	e, err := c.tree.EntryByPath(key)
	if err != nil {
		return out, err
	}
	blob, err := lookupBlob(c.db.repo, e.Id)
	if err != nil {
		return out, err
	}
	defer blob.Free()
	return blob.Contents(), err
}

func (c *Collection) Push(url string) error {
	// The '+' prefix sets force=true,
	// so the remote ref is created if it doesn't exist.
	refspec := fmt.Sprintf("+ref/heads/%s:%s", c.name)

	remote, err := c.db.repo.Remotes.CreateAnonymous(url)
	if err != nil {
		return err
	}

	err = remote.Push([]string{refspec}, &git.PushOptions{})
	if err != nil {
		return fmt.Errorf("git_push_new: %v", err)
	}
	return nil
}

func (c *Collection) ref() string {
	return fmt.Sprintf("refs/heads/%s", c.name)
}

func (c *Collection) Pull(url string) error {
	c.Lock()
	defer c.Unlock()

	return c.pullUnsafe(url)
}

func (c *Collection) pullUnsafe(url string) error {
	refspec := strings.Join([]string{c.ref(), c.ref()}, ":")

	remote, err := c.db.repo.Remotes.CreateAnonymous(url)
	if err != nil {
		return err
	}
	defer remote.Free()
	c.resetUnsafe()

	if err := remote.Fetch([]string{refspec}, &git.FetchOptions{}, fmt.Sprintf("libpack.pull %s %s", url, refspec)); err != nil {
		return err
	}

	return nil
}

func (c *Collection) Update(url string) error {
	catcher := grip.NewCatcher()

	catcher.Add(c.Pull(url))
	catcher.Add(c.Reset())

	return catcher.Resolve()
}

func (c *Collection) Reset() error {
	c.Lock()
	defer c.Unlock()

	return c.resetUnsafe()
}

func (c *Collection) resetUnsafe() error {
	tip, err := c.db.repo.References.Lookup(c.name)
	if err != nil {
		if c.commit != nil {
			c.commit.Free()
		}

		c.commit = nil
		return err
	}

	if c.commit != nil && c.commit.Id().Equal(tip.Target()) {
		// we have the latest commit and there's nothing
		// staged.
		return nil
	}

	tipCommit, err := c.db.repo.LookupCommit(tip.Target())
	if err != nil {
		return err
	}
	cleanTree, err := tipCommit.Tree()
	if err != nil {
		return err
	}

	if c.tree != nil {
		c.tree.Free()
	}
	c.tree = cleanTree

	if c.commit != nil {
		c.commit.Free()
	}
	c.commit = tipCommit

	return nil
}

func (c *Collection) setCommit() error {
	tip, err := c.db.repo.References.Lookup(c.name)
	if err != nil {
		return err
	}

	tipCommit, err := c.db.repo.LookupCommit(tip.Target())

	if c.commit != nil {
		c.commit.Free()
	}
	c.commit = tipCommit
	return nil
}
