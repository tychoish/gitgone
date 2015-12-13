package store

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/tychoish/grip"
)

type opType int

const (
	opAdd opType = iota
	opDel

//	opUpdate
)

type operation struct {
	key    string
	value  interface{}
	action opType
}

type Transaction struct {
	coll            *Collection
	ops             []*operation
	deleteGroup     []string
	ContinueOnError bool
	sync.Mutex
}

func (c *Collection) NewTransaction() *Transaction {
	return &Transaction{
		coll: c,
	}
}

func (txn *Transaction) Add(key string, value interface{}) {
	txn.Lock()
	defer txn.Unlock()

	txn.ops = append(txn.ops, &operation{key, value, opAdd})
}

func (txn *Transaction) Delete(key string, value interface{}) {
	txn.Lock()
	defer txn.Unlock()

	txn.ops = append(txn.ops, &operation{key, value, opDel})
}

func (txn *Transaction) RemoveOperation(key string, value interface{}) {
	txn.Lock()
	defer txn.Unlock()

	for idx, op := range txn.ops {
		if key == op.key && value == op.value {
			txn.ops = append(txn.ops[:idx], txn.ops[idx+1:]...)
		}
	}
}

func (op *operation) convertData() ([]byte, error) {
	switch v := op.value.(type) {
	default:
		return json.Marshal(v)
	case byte:
		return []byte{v}, nil
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case int:
		return []byte(strconv.Itoa(v)), nil
	case bool:
		return []byte(strconv.FormatBool(v)), nil
	}
}

func (txn *Transaction) removeDeleteGroup() error {
	if len(txn.deleteGroup) == 0 {
		return nil
	}

	newTree, err := treeRemove(txn.coll.db.repo, txn.coll.tree, txn.deleteGroup)
	if err == nil || txn.ContinueOnError {
		txn.deleteGroup = []string{}
		txn.coll.tree = newTree
	}

	return err
}

func (txn *Transaction) Run() (err error) {
	// get the transaction lock, and then the collection lock
	txn.Lock()
	defer txn.Unlock()
	txn.coll.Lock()
	defer txn.coll.Unlock()

	catcher := grip.NewCatcher()

	for _, op := range txn.ops {
		if op.action == opAdd {
			// delete any pending deletes in a group before adding any ops.
			err = txn.removeDeleteGroup()
			catcher.Add(err)
			if err != nil && !txn.ContinueOnError {
				return catcher.Resolve()
			}

			value, err := op.convertData()
			catcher.Add(err)
			if err != nil && !txn.ContinueOnError {
				return catcher.Resolve()
			}

			id, err := txn.coll.db.repo.CreateBlobFromBuffer(value)
			catcher.Add(err)
			if err == nil {
				newTree, err := treeAdd(txn.coll.db.repo, txn.coll.tree, op.key, id)
				catcher.Add(err)
				if err != nil && !txn.ContinueOnError {
					catcher.Add(txn.coll.resetUnsafe())
					return catcher.Resolve()
				}
				txn.coll.tree = newTree
			}
		} else if op.action == opDel {
			// it's easy and safe to group deletes. Therefore, we just add to the delete group during processing.
			txn.deleteGroup = append(txn.deleteGroup, op.key)
		}
	}

	// attempt to run the last delete group incase there are items when the loop ends.
	catcher.Add(txn.removeDeleteGroup())

	if catcher.HasErrors() {
		txn.deleteGroup = []string{}
		return catcher.Resolve()
	}

	if len(txn.ops) > 0 && txn.coll.tree != nil {
		err = txn.coll.commitUnsafe(fmt.Sprintf("added %d ops in 1 commit", len(txn.ops)))
		catcher.Add(err)
	}

	txn.ops = []*operation{}
	if len(txn.deleteGroup) > 0 {
		txn.deleteGroup = []string{}
	}

	return catcher.Resolve()
}
