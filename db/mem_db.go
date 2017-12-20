package db

import (
	"fmt"
	"sort"
	"sync"
)

func init() {
	registerDBCreator(MemDBBackendStr, func(name string, dir string) (DB, error) {
		return NewMemDB(), nil
	}, false)
}

var _ DB = (*MemDB)(nil)

type MemDB struct {
	mtx sync.Mutex
	db  map[string][]byte
}

func NewMemDB() *MemDB {
	database := &MemDB{
		db: make(map[string][]byte),
	}
	return database
}

func (db *MemDB) Get(key []byte) []byte {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = nonNilBytes(key)
  
	return db.db[string(key)]
}

func (db *MemDB) Has(key []byte) bool {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	key = nonNilBytes(key)

	_, ok := db.db[string(key)]
	return ok
}

func (db *MemDB) Set(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

func (db *MemDB) SetSync(key []byte, value []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.SetNoLock(key, value)
}

// NOTE: Implements atomicSetDeleter
func (db *MemDB) SetNoLock(key []byte, value []byte) {
	key = nonNilBytes(key)
	value = nonNilBytes(value)

	db.db[string(key)] = value
}

func (db *MemDB) Delete(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}

func (db *MemDB) DeleteSync(key []byte) {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	db.DeleteNoLock(key)
}

// NOTE: Implements atomicSetDeleter
func (db *MemDB) DeleteNoLock(key []byte) {
	key = nonNilBytes(key)

	delete(db.db, string(key))
}

func (db *MemDB) Close() {
	// Close is a noop since for an in-memory
	// database, we don't have a destination
	// to flush contents to nor do we want
	// any data loss on invoking Close()
	// See the discussion in https://github.com/tendermint/tmlibs/pull/56
}

func (db *MemDB) Print() {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	for key, value := range db.db {
		fmt.Printf("[%X]:\t[%X]\n", []byte(key), value)
	}
}

func (db *MemDB) Stats() map[string]string {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	stats := make(map[string]string)
	stats["database.type"] = "memDB"
	stats["database.size"] = fmt.Sprintf("%d", len(db.db))
	return stats
}

func (db *MemDB) NewBatch() Batch {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	return &memBatch{db, nil}
}

func (db *MemDB) Mutex() *sync.Mutex {
	return &(db.mtx)
}

//----------------------------------------

func (db *MemDB) Iterator(start, end []byte) Iterator {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	keys := db.getSortedKeys(start, end, false)
	return newMemDBIterator(db, keys, start, end)
}

func (db *MemDB) ReverseIterator(start, end []byte) Iterator {
	db.mtx.Lock()
	defer db.mtx.Unlock()

	keys := db.getSortedKeys(end, start, true)
	return newMemDBIterator(db, keys, start, end)
}

func (db *MemDB) getSortedKeys(start, end []byte, reverse bool) []string {
	keys := []string{}
	for key, _ := range db.db {
		if IsKeyInDomain([]byte(key), start, end, false) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if reverse {
		nkeys := len(keys)
		for i := 0; i < nkeys/2; i++ {
			keys[i] = keys[nkeys-i-1]
		}
	}
	return keys
}

var _ Iterator = (*memDBIterator)(nil)

// We need a copy of all of the keys.
// Not the best, but probably not a bottleneck depending.
type memDBIterator struct {
	db    DB
	cur   int
	keys  []string
	start []byte
	end   []byte
}

// Keys is expected to be in reverse order for reverse iterators.
func newMemDBIterator(db DB, keys []string, start, end []byte) *memDBIterator {
	return &memDBIterator{
		db:    db,
		cur:   0,
		keys:  keys,
		start: start,
		end:   end,
	}
}

func (itr *memDBIterator) Domain() ([]byte, []byte) {
	return itr.start, itr.end
}

func (itr *memDBIterator) Valid() bool {
	return 0 <= itr.cur && itr.cur < len(itr.keys)
}

func (itr *memDBIterator) Next() {
	itr.assertIsValid()
	itr.cur++
}

func (itr *memDBIterator) Key() []byte {
	itr.assertIsValid()
	return []byte(itr.keys[itr.cur])
}

func (itr *memDBIterator) Value() []byte {
	itr.assertIsValid()
	key := []byte(itr.keys[itr.cur])
	return itr.db.Get(key)
}

func (itr *memDBIterator) Close() {
	itr.keys = nil
	itr.db = nil
}

func (itr *memDBIterator) assertIsValid() {
	if !itr.Valid() {
		panic("memDBIterator is invalid")
	}
}