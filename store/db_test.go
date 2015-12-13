package store

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type DBSuite struct {
	dir      string
	collName string
	count    int
	db       *Database
}

var _ = Suite(&DBSuite{})

func (s *DBSuite) SetUpSuite(c *C) {
	s.count = 128

	path, err := ioutil.TempDir("", "db-tests")
	c.Assert(err, IsNil)
	s.dir = path

	db, err := NewDatabase(s.dir)
	c.Assert(err, IsNil)
	s.db = db

	s.collName = "foo"
}
func (s *DBSuite) TearDownSuite(c *C) {
	os.RemoveAll(s.dir)
}

func (s *DBSuite) TestDBCreationAndCaching(c *C) {
	c.Logf("testing if attempts to create an existing db will hit the cache")
	for i := 1; i <= s.count; i++ {
		db, err := NewDatabase(s.dir)
		c.Assert(err, IsNil)
		c.Assert(db.path, Equals, s.dir)
		c.Assert(len(dbCache.dbs), Equals, 1)

	}
	c.Assert(len(dbCache.dbs), Equals, 1)

	c.Logf("testing concurrent creation/access to one database.")
	wg := &sync.WaitGroup{}
	for i := 1; i <= s.count; i++ {
		wg.Add(1)
		go func() {
			db, err := NewDatabase(s.dir)
			c.Assert(err, IsNil)
			c.Assert(db.path, Equals, s.dir)
			c.Assert(len(dbCache.dbs), Equals, 1)
			wg.Done()
		}()
	}
	wg.Wait()

	c.Logf("testing concurrent creation/access of multiple databases.")
	for i := 1; i <= s.count; i++ {
		wg.Add(1)
		go func(num int) {
			localDbPath := filepath.Join(s.dir, strconv.Itoa(num))
			db, err := NewDatabase(localDbPath)
			c.Assert(err, IsNil)
			c.Assert(db.path, Equals, localDbPath)
			wg.Done()
		}(i)
	}
	wg.Wait()

	c.Assert(len(dbCache.dbs), Equals, s.count+1)
}

func (s *DBSuite) TestCollectionCreationAndCaching(c *C) {
	db, err := NewDatabase(s.dir)
	c.Assert(err, IsNil)

	for i := 1; i <= s.count; i++ {
		_ = db.Collection(s.collName)
		c.Assert(len(db.collections), Equals, 1)
	}

	for i := 1; i <= s.count; i++ {
		_ = db.Collection("one" + strconv.Itoa(i))
	}
	c.Assert(len(db.collections), Equals, s.count+1)

	// run in two loops to make sure the first time we create a
	// bunch of collections, and the second time we just pull them
	// out of the cache.
	c.Logf("parallel collection creation uses the collection cache")
	wg := &sync.WaitGroup{}
	for o := 1; o <= 2; o++ {
		for i := 1; i <= s.count; i++ {
			wg.Add(1)
			go func(num int) {
				_ = db.Collection("two" + strconv.Itoa(num))
				wg.Done()
			}(i)
		}
		wg.Wait()
		c.Assert(len(db.collections), Equals, s.count+s.count+1)
	}
	c.Assert(len(db.collections), Equals, s.count+s.count+1)

	c.Logf("list collections generator returns all created collections")
	collections := []string{}
	for coll := range db.ListCollections() {
		collections = append(collections, coll)
	}
	c.Assert(len(collections), Equals, s.count+s.count+1)
}
