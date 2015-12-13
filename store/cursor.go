package store

import (
	"github.com/tychoish/grip"
	"gopkg.in/libgit2/git2go.v23"
)

type Record struct {
	Name  string
	Value []byte
}

func (self *Collection) SnapshotTableScanCursor() chan *Record {
	c := make(chan *Record)

	self.RLock()
	scanTree := self.tree
	self.RUnlock()
	go func() {
		scanTree.Walk(func(root string, tree *git.TreeEntry) int {

			if tree.Type == git.ObjectBlob {
				blob, err := self.db.repo.LookupBlob(tree.Id)
				if err != nil {
					blob.Free()
					grip.CatchError(err)
					return 0
				}

				c <- &Record{
					Name:  tree.Name,
					Value: blob.Contents(),
				}

				blob.Free()
			} else {
				grip.Info(tree.Name)
			}

			return 0
		})

		close(c)
	}()

	return c
}
