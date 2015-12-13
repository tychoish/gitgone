package store

// These functions are from libpack-1
// <https://github.com/shykes/libpack-1> which provide helpers on top
// of libgit2 for some common operations. I've added some additional
// logging, and think that at some point it might make sense to
// refactor/remove most of these, but they're all internal, so it's safe unt

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/tychoish/grip"
	"gopkg.in/libgit2/git2go.v23"
)

func treePath(p string) string {
	p = path.Clean(p)
	if p == "/" || p == "." {
		return "/"
	}
	// Remove leading / from the path
	// as libgit2.TreeEntryByPath does not accept it
	p = strings.TrimLeft(p, "/")
	return p
}

// treeAdd creates a new Git tree by adding a new object
// to it at the specified path.
// Intermediary subtrees are created as needed.
// If an object already exists at key or any intermediary path,
// it is overwritten. New trees are merged into existing ones at the
// file granularity (similar to 'cp -R').
//
// Since git trees are immutable, base is not modified. The new
// tree is returned.
// If an error is encountered, intermediary objects may be left
// behind in the git repository. It is the caller's responsibility
// to perform garbage collection, if any.
// FIXME: manage garbage collection, or provide a list of created
// objects.
func treeAdd(repo *git.Repository, tree *git.Tree, key string, valueId *git.Oid) (t *git.Tree, err error) {
	/*
	** // Primitive but convenient tracing for debugging recursive calls to treeAdd.
	** // Uncomment this block for debug output.
	**
	** var callString string
	** if tree != nil {
	**		callString = fmt.Sprintf("   treeAdd %v:\t\t%s\t\t\t= %v", tree.Id(), key, valueId)
	**	} else {
	**		callString = fmt.Sprintf("   treeAdd %v:\t\t%s\t\t\t= %v", tree, key, valueId)
	**	}
	**	fmt.Printf("   %s\n", callString)
	**	defer func() {
	**		if t != nil {
	**			fmt.Printf("-> %s => %v\n", callString, t.Id())
	**		} else {
	**			fmt.Printf("-> %s => %v\n", callString, err)
	**		}
	**	}()
	 */
	if valueId == nil {
		return tree, nil
	}
	key = treePath(key)
	base, leaf := path.Split(key)
	o, err := repo.Lookup(valueId)
	if err != nil {
		return nil, err
	}
	var builder *git.TreeBuilder
	if tree == nil {
		builder, err = repo.TreeBuilder()
		if err != nil {
			return nil, err
		}
	} else {
		builder, err = repo.TreeBuilderFromTree(tree)
		if err != nil {
			return nil, err
		}
	}
	defer builder.Free()
	// The specified path has only 1 component (the "leaf")
	if base == "" || base == "/" {
		// If val is a string, set it and we're done.
		// Any old value is overwritten.
		if _, isBlob := o.(*git.Blob); isBlob {
			if err := builder.Insert(leaf, valueId, 0100644); err != nil {
				return nil, err
			}
			newTreeId, err := builder.Write()
			if err != nil {
				return nil, err
			}
			newTree, err := lookupTree(repo, newTreeId)
			if err != nil {
				return nil, err
			}
			return newTree, nil
		}
		// If val is not a string, it must be a subtree.
		// Return an error if it's any other type than Tree.
		oTree, ok := o.(*git.Tree)
		if !ok {
			return nil, fmt.Errorf("value must be a blob or subtree")
		}
		var subTree *git.Tree
		var oldSubTree *git.Tree
		if tree != nil {
			oldSubTree, err = treeScope(repo, tree, leaf)
			// FIXME: distinguish "no such key" error (which
			// FIXME: distinguish a non-existing previous tree (continue with oldTree==nil)
			// from other errors (abort and return an error)
			if err == nil {
				defer oldSubTree.Free()
			}
		}
		// If that subtree already exists, merge the new one in.
		if oldSubTree != nil {
			subTree = oldSubTree
			for i := uint64(0); i < oTree.EntryCount(); i++ {
				var err error
				e := oTree.EntryByIndex(i)
				subTree, err = treeAdd(repo, subTree, e.Name, e.Id)
				if err != nil {
					return nil, err
				}
			}
		}
		// If the key is /, we're replacing the current tree
		if key == "/" {
			return subTree, nil
		}
		// Otherwise we're inserting into the current tree
		if err := builder.Insert(leaf, subTree.Id(), 040000); err != nil {
			return nil, err
		}
		newTreeId, err := builder.Write()
		if err != nil {
			return nil, err
		}
		newTree, err := lookupTree(repo, newTreeId)
		if err != nil {
			return nil, err
		}
		return newTree, nil
	}
	subtree, err := treeAdd(repo, nil, leaf, valueId)
	if err != nil {
		return nil, err
	}
	return treeAdd(repo, tree, base, subtree.Id())
}

func treeRemove(repo *git.Repository, tree *git.Tree, keys []string) (t *git.Tree, err error) {
	builder, err := repo.TreeBuilderFromTree(tree)
	if err != nil {
		return tree, err
	}
	defer builder.Free()

	catcher := grip.NewCatcher()
	for _, fn := range keys {
		catcher.Add(builder.Remove(fn))
	}
	if catcher.HasErrors() {
		return tree, catcher.Resolve()
	}

	newTreeId, err := builder.Write()
	if err != nil {
		return nil, err
	}

	t, err = lookupTree(repo, newTreeId)
	if err != nil {
		return nil, err
	}
	return
}

func treeGet(r *git.Repository, t *git.Tree, key string) (string, error) {
	if t == nil {
		return "", os.ErrNotExist
	}
	key = treePath(key)
	e, err := t.EntryByPath(key)
	if err != nil {
		return "", err
	}
	blob, err := lookupBlob(r, e.Id)
	if err != nil {
		return "", err
	}
	defer blob.Free()
	return string(blob.Contents()), nil

}

func treeList(r *git.Repository, t *git.Tree, key string) ([]string, error) {
	if t == nil {
		return []string{}, nil
	}
	subtree, err := treeScope(r, t, key)
	if err != nil {
		return nil, err
	}
	defer subtree.Free()
	var (
		i     uint64
		count uint64 = subtree.EntryCount()
	)
	entries := make([]string, 0, count)
	for i = 0; i < count; i++ {
		entries = append(entries, subtree.EntryByIndex(i).Name)
	}
	return entries, nil
}

func treeWalk(r *git.Repository, t *git.Tree, key string, h func(string, git.Object) error) error {
	if t == nil {
		return fmt.Errorf("no tree to walk")
	}
	subtree, err := treeScope(r, t, key)
	if err != nil {
		return err
	}
	var handlerErr error
	err = subtree.Walk(func(parent string, e *git.TreeEntry) int {
		obj, err := r.Lookup(e.Id)
		if err != nil {
			handlerErr = err
			return -1
		}
		if err := h(path.Join(parent, e.Name), obj); err != nil {
			handlerErr = err
			return -1
		}
		obj.Free()
		return 0
	})
	if handlerErr != nil {
		return handlerErr
	}
	if err != nil {
		return err
	}
	return nil
}

func treeDump(r *git.Repository, t *git.Tree, key string, dst io.Writer) error {
	return treeWalk(r, t, key, func(key string, obj git.Object) error {
		if _, isTree := obj.(*git.Tree); isTree {
			fmt.Fprintf(dst, "%s/\n", key)
		} else if blob, isBlob := obj.(*git.Blob); isBlob {
			fmt.Fprintf(dst, "%s = %s\n", key, blob.Contents())
		}
		return nil
	})
}

func treeScope(repo *git.Repository, tree *git.Tree, name string) (*git.Tree, error) {
	if tree == nil {
		return nil, fmt.Errorf("tree undefined")
	}
	name = treePath(name)
	if name == "/" {
		// Allocate a new Tree object so that the caller
		// can always call Free() on the result
		return lookupTree(repo, tree.Id())
	}
	entry, err := tree.EntryByPath(name)
	if err != nil {
		return nil, err
	}
	return lookupTree(repo, entry.Id)
}

//

func lookupTree(r *git.Repository, id *git.Oid) (*git.Tree, error) {
	obj, err := r.Lookup(id)
	if err != nil {
		return nil, err
	}
	if tree, ok := obj.(*git.Tree); ok {
		return tree, nil
	}
	return nil, fmt.Errorf("hash %v exist but is not a tree", id)
}

// lookupBlob looks up an object at hash `id` in `repo`, and returns
// it as a git blob. If the object is not a blob, an error is returned.
func lookupBlob(r *git.Repository, id *git.Oid) (*git.Blob, error) {
	obj, err := r.Lookup(id)
	if err != nil {
		return nil, err
	}
	if blob, ok := obj.(*git.Blob); ok {
		return blob, nil
	}
	return nil, fmt.Errorf("hash %v exist but is not a blob", id)
}

// lookupTip looks up the object referenced by refname, and returns it
// as a Commit object. If the reference does not exist, or if object is
// not a commit, nil is returned. Other errors cannot be detected.
func lookupTip(r *git.Repository, refname string) *git.Commit {
	ref, err := r.References.Lookup(refname)
	if err != nil {
		return nil
	}
	commit, err := lookupCommit(r, ref.Target())
	if err != nil {
		return nil
	}
	return commit
}

// lookupCommit looks up an object at hash `id` in `repo`, and returns
// it as a git commit. If the object is not a commit, an error is returned.
func lookupCommit(r *git.Repository, id *git.Oid) (*git.Commit, error) {
	obj, err := r.Lookup(id)
	if err != nil {
		return nil, err
	}
	if commit, ok := obj.(*git.Commit); ok {
		return commit, nil
	}
	return nil, fmt.Errorf("hash %v exist but is not a commit", id)
}

func isGitIterOver(err error) bool {
	gitErr, ok := err.(*git.GitError)
	if !ok {
		return false
	}
	return gitErr.Code == git.ErrIterOver
}
