package store

// more legacy tests taken from libpack-1 ported to gocheck.

import (
	"io/ioutil"
	"os"

	. "gopkg.in/check.v1"
	"gopkg.in/libgit2/git2go.v23"
)

type TreeSuite struct {
	repo *git.Repository
	path string
}

var _ = Suite(&TreeSuite{})

func (s *TreeSuite) SetUpSuite(c *C) {
	dir, err := ioutil.TempDir("", "gitgone-store-test-legacy-libpac-")
	if err != nil {
		c.Fatal(err)
	}
	s.path = dir

	repo, err := git.InitRepository(dir, true)
	if err != nil {
		c.Fatal(err)
	}

	s.repo = repo

}

func (s *TreeSuite) TearDownSuite(c *C) {
	s.repo.Free()
	os.RemoveAll(s.path)
}

func (s *TreeSuite) TestEmptyTree(c *C) {
	empty, err := emptyTree(s.repo)
	if err != nil {
		c.Fatal(err)
	}
	if empty.String() != "4b825dc642cb6eb9a060e54bf8d69288fbee4904" {
		c.Fatalf("%v", empty)
	}
}

func (s *TreeSuite) TestUpdateTree1(c *C) {
	hello, err := s.repo.CreateBlobFromBuffer([]byte("hello"))
	if err != nil {
		c.Fatal(err)
	}
	emptyId, _ := emptyTree(s.repo)
	empty, _ := lookupTree(s.repo, emptyId)
	t1, err := treeAdd(s.repo, empty, "foo", hello)
	if err != nil {
		c.Fatal(err)
	}
	assertBlobInTree(c, s.repo, t1, "foo", "hello")

	t1b, err := treeAdd(s.repo, empty, "bar", hello)
	if err != nil {
		c.Fatal(err)
	}

	t2b, err := treeAdd(s.repo, t1, "/", t1b.Id())
	if err != nil {
		c.Fatal(err)
	}
	assertBlobInTree(c, s.repo, t2b, "foo", "hello")
	assertBlobInTree(c, s.repo, t2b, "bar", "hello")
}

func assertBlobInTree(c *C, repo *git.Repository, tree *git.Tree, key, value string) {
	e, err := tree.EntryByPath(key)
	if err != nil || e == nil {
		c.Fatalf("No blob at key %v.\n\ttree=%#v\n", key, tree)
	}
	blob, err := lookupBlob(repo, e.Id)
	if err != nil {
		c.Fatalf("No blob at key %v.\\n\terr=%v\n\ttree=%#v\n", key, err, tree)
	}
	if string(blob.Contents()) != value {
		c.Fatalf("blob at key %v != %v.\n\ttree=%#v\n\treal val = %v\n", key, value, tree, string(blob.Contents()))
	}
	blob.Free()
}
