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
		return db, nil
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

		self.dbs[path] = db
		return db, err
	}
}

func NewDatabase(path string) (*Database, error) {
	return dbCache.getDatabase(path)
}

func (db *Database) ListCollections() chan string {
	db.RLock()
	defer db.RUnlock()

	c := make(chan string)

	go func() {
		for coll := range db.collections {
			c <- coll
		}
		close(c)
	}()

	return c
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
