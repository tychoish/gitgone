package store

import (
	"os"
	"sync"

	"github.com/tychoish/grip"
	"gopkg.in/libgit2/git2go.v23"
)

type Database struct {
	path string
	repo *git.Repository
	sync.RWMutex
	collections map[string]*Collection
}

type dbCacheTracker struct {
	dbs map[string]*Database
	sync.Mutex
}

var dbCache *dbCacheTracker

func init() {
	dbCache = &dbCacheTracker{
		dbs: make(map[string]*Database),
	}
}

func (self *dbCacheTracker) getDatabase(path string) (*Database, error) {
	self.Lock()
	defer self.Unlock()

	db, ok := self.dbs[path]
	if ok {
		err := db.loadCollections()
		return db, err
	} else {
		repo, err := git.OpenRepository(path)
		if err != nil {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				err = os.MkdirAll(path, 0755)
				if err != nil {
					return nil, err
				}
			}
			repo, err = git.InitRepository(path, true)
		}

		db := &Database{
			collections: make(map[string]*Collection),
			path:        path,
			repo:        repo,
		}

		if err == nil {
			self.dbs[path] = db
			err = db.loadCollections()
		}
		return db, err
	}
}

func (self *dbCacheTracker) removeDatabae(path string, hard bool) error {
	self.Lock()
	defer self.Unlock()

	db, ok := self.dbs[path]
	if !ok {
		return nil
	} else {
		db.Lock()
		defer db.Unlock()
		if hard {
			os.RemoveAll(db.path)
			delete(self.dbs, path)
			db.repo.Free()
			return nil
		} else {
			delete(self.dbs, path)
			return nil
		}
	}
}

func NewDatabase(path string) (*Database, error) {
	db, err := dbCache.getDatabase(path)
	if err != nil {
		return db, err
	}
	err = db.loadCollections()
	return db, err
}

func (db *Database) Remove(hard bool) {
	dbCache.removeDatabae(db.path, hard)
}

func (db *Database) ListCollections() chan string {
	c := make(chan string)

	db.RLock()
	defer db.RUnlock()

	go func() {
		for coll := range db.collections {
			c <- coll
		}
		close(c)
	}()

	return c
}

func (db *Database) loadCollections() error {
	db.Lock()
	defer db.Unlock()

	iter, err := db.repo.NewReferenceNameIterator()
	if err != nil {
		return err
	}

	ref, err := iter.Next()
	for err == nil {
		coll := db.createCollectionUnsafe(ref)
		db.collections[ref] = coll

		// advance the iterator
		ref, err = iter.Next()
	}
	return nil
}

func (db *Database) Update() error {
	catcher := grip.NewCatcher()
	for _, coll := range db.collections {
		catcher.Add(coll.Reset())
	}
	return catcher.Resolve()
}

func (db *Database) Pull(url string) error {
	catcher := grip.NewCatcher()
	for _, coll := range db.collections {
		catcher.Add(coll.Pull(url))
	}
	return catcher.Resolve()
}

func (db *Database) Push(url string) error {
	catcher := grip.NewCatcher()
	for _, coll := range db.collections {
		catcher.Add(coll.Push(url))
	}
	return catcher.Resolve()
}
