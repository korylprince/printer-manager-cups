package cache

import (
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
)

//Cache an expiring value store
type Cache map[string]time.Time

//Read returns the Cache from the given path, or an error if one occurred
func Read(path string) (Cache, error) {
	db, err := badger.Open(badger.DefaultOptions(path).WithLogger(nil))
	if err != nil {
		return nil, fmt.Errorf("Unable to open db: %v", err)
	}
	defer db.Close()

	cache := make(Cache)

	if err = db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			if err := item.Value(func(v []byte) error {
				t := new(time.Time)
				err := t.UnmarshalBinary(v)
				if err != nil {
					return fmt.Errorf("Unable to unmarshal time: %v", err)
				}
				cache[string(item.Key())] = *t
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("Unable to read iterate through database: %v", err)
	}

	return cache, nil
}

//Write writes cache to the given path or returns an error if one occurred
func (cache Cache) Write(path string) error {
	db, err := badger.Open(badger.DefaultOptions(path).WithLogger(nil))
	if err != nil {
		return fmt.Errorf("Unable to open db: %v", err)
	}
	defer db.Close()

	if err := db.Update(func(txn *badger.Txn) error {
		for id, t := range cache {
			tb, err := t.MarshalBinary()
			if err != nil {
				return fmt.Errorf("Unable to marshal time: %v", err)
			}
			if err := txn.Set([]byte(id), tb); err != nil {
				return fmt.Errorf("Unable to write to cache: %v", err)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}

//Purge removes the given ids from the cache at the given path, or returns an error if one occurred
func Purge(path string, ids []string) error {
	db, err := badger.Open(badger.DefaultOptions(path).WithLogger(nil))
	if err != nil {
		return fmt.Errorf("Unable to open db: %v", err)
	}
	defer db.Close()

	if err := db.Update(func(txn *badger.Txn) error {
		for _, id := range ids {
			if err := txn.Delete([]byte(id)); err != nil {
				return fmt.Errorf("Unable to delete key: %v", err)
			}
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}
