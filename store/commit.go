package store

import (
	"fmt"
	"time"

	"gopkg.in/libgit2/git2go.v23"
)

func (c *Collection) Commit(msg string) error {
	c.Lock()
	defer c.Unlock()

	return c.commitUnsafe(msg)
}

func (c *Collection) commitUnsafe(msg string) error {
	if c.tree == nil {
		// Nothing to commit
		return nil
	}

	commit, err := commitToRef(c.db.repo, c.tree, c.commit, c.ref(), msg)
	if err != nil {
		return err
	}

	if c.commit != nil {
		c.commit.Free()
	}
	c.commit = commit
	return nil
}

// these operations are taken from: https://github.com/shykes/libpack-1
func commitToRef(r *git.Repository, tree *git.Tree, parent *git.Commit, refname, msg string) (*git.Commit, error) {
	// Retry loop in case of conflict
	// FIXME: use a custom inter-process lock as a first attempt for performance
	var (
		needMerge bool
		tmpCommit *git.Commit
	)
	for {
		if !needMerge {
			// Create simple commit
			commit, err := mkCommit(r, refname, msg, tree, parent)
			if isGitConcurrencyErr(err) {
				needMerge = true
				continue
			}
			return commit, err
		} else {
			if tmpCommit == nil {
				var err error
				// Create a temporary intermediary commit, to pass to MergeCommits
				// NOTE: this commit will not be part of the final history.
				tmpCommit, err = mkCommit(r, "", msg, tree, parent)
				if err != nil {
					return nil, err
				}
				defer tmpCommit.Free()
			}
			// Lookup tip from ref
			tip := lookupTip(r, refname)
			if tip == nil {
				// Ref may have been deleted after previous merge error
				needMerge = false
				continue
			}

			// Merge simple commit with the tip
			opts, err := git.DefaultMergeOptions()
			if err != nil {
				return nil, err
			}
			idx, err := r.MergeCommits(tmpCommit, tip, &opts)
			if err != nil {
				return nil, err
			}
			conflicts, err := idx.ConflictIterator()
			if err != nil {
				return nil, err
			}
			defer conflicts.Free()
			for {
				c, err := conflicts.Next()
				if isGitIterOver(err) {
					break
				} else if err != nil {
					return nil, err
				}
				if c.Our != nil {
					idx.RemoveConflict(c.Our.Path)
					if err := idx.Add(c.Our); err != nil {
						return nil, fmt.Errorf("error resolving merge conflict for '%s': %v", c.Our.Path, err)
					}
				}
			}
			mergedId, err := idx.WriteTreeTo(r)
			if err != nil {
				return nil, fmt.Errorf("WriteTree: %v", err)
			}
			mergedTree, err := lookupTree(r, mergedId)
			if err != nil {
				return nil, err
			}
			// Create new commit from merged tree (discarding simple commit)
			commit, err := mkCommit(r, refname, msg, mergedTree, parent, tip)
			if isGitConcurrencyErr(err) {
				// FIXME: enforce a maximum number of retries to avoid infinite loops
				continue
			}
			return commit, err
		}
	}
	return nil, fmt.Errorf("too many failed merge attempts, giving up")
}

func mkCommit(r *git.Repository, refname string, msg string, tree *git.Tree, parent *git.Commit, extraParents ...*git.Commit) (*git.Commit, error) {
	var parents []*git.Commit
	if parent != nil {
		parents = append(parents, parent)
	}
	if len(extraParents) > 0 {
		parents = append(parents, extraParents...)
	}
	id, err := r.CreateCommit(
		refname,
		&git.Signature{"libpack", "libpack", time.Now()}, // author
		&git.Signature{"libpack", "libpack", time.Now()}, // committer
		msg,
		tree, // git tree to commit
		parents...,
	)
	if err != nil {
		return nil, err
	}
	return lookupCommit(r, id)
}

func isGitConcurrencyErr(err error) bool {
	gitErr, ok := err.(*git.GitError)
	if !ok {
		return false
	}
	return gitErr.Class == 11 && gitErr.Code == -15
}
