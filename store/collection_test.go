package store

import (
	"strconv"
	"strings"
	"sync"

	. "gopkg.in/check.v1"
)

func (s *DBSuite) TestSimpleRoundTrip(c *C) {
	coll := s.db.Collection(s.collName)
	txn := coll.NewTransaction()
	txn.Add("foo", "bar")
	err := txn.Run()
	c.Assert(err, IsNil)

	out, err := coll.Get("foo")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "bar")
}

func (s *DBSuite) TestConcurrentReads(c *C) {
	coll := s.db.Collection(s.collName)
	txn := coll.NewTransaction()
	txn.Add("read-test", "02/02/02")
	err := txn.Run()
	c.Assert(err, IsNil)

	wg := &sync.WaitGroup{}
	for i := 1; i <= s.count; i++ {
		wg.Add(1)
		go func() {
			value, err := coll.Get("read-test")
			c.Assert(err, IsNil)
			c.Assert(string(value), Equals, "02/02/02")
			wg.Done()
		}()
	}
	wg.Wait()
}

func (s *DBSuite) TestConcurrentInserts(c *C) {
	coll := s.db.Collection(s.collName)

	wg := &sync.WaitGroup{}
	for i := 1; i <= s.count; i++ {
		wg.Add(1)
		go func(num string) {
			// declare values
			keyName := strings.Join([]string{"write", "test", num}, "-")
			value := strings.Join([]string{num, num, num}, "/")

			// add the keys
			txn := coll.NewTransaction()
			txn.Add(keyName, value)
			err := txn.Run()
			c.Assert(err, IsNil)

			// geth them out and make sure they match
			dbValue, err := coll.Get(keyName)
			c.Assert(err, IsNil)
			c.Assert(string(dbValue), Equals, value)

			wg.Done()
		}(strconv.Itoa(i))
	}
	s.db.loadCollections()
	wg.Wait()
}

func (s *DBSuite) TestConcurrentTransactionAdds(c *C) {
	coll := s.db.Collection(s.collName)
	txn := coll.NewTransaction()

	wg := &sync.WaitGroup{}
	for i := 1; i <= s.count; i++ {
		wg.Add(1)
		go func(num string) {
			// declare values
			keyName := strings.Join([]string{"write", "test", num}, "-")
			value := strings.Join([]string{num, num, num}, "/")

			// add the keys
			txn.Add(keyName, value)
			wg.Done()
		}(strconv.Itoa(i))

	}
	wg.Wait()
	c.Assert(len(txn.ops), Equals, s.count)

	err := txn.Run()
	c.Assert(err, IsNil)
	c.Assert(len(txn.ops), Equals, 0)
}
