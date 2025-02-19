package db

import (
	"math/big"
	"sync"

	"github.com/tenderly/zkevm-erigon/smt/pkg/utils"
)

type MemDb struct {
	Db       map[string][]string
	DbAccVal map[string][]string
	LastRoot *big.Int

	lock sync.RWMutex
}

func NewMemDb() *MemDb {
	return &MemDb{
		Db:       make(map[string][]string),
		DbAccVal: make(map[string][]string),
		LastRoot: big.NewInt(0),
	}
}

func (m *MemDb) OpenBatch(quitCh <-chan struct{}) {
}

func (m *MemDb) CommitBatch() error {
	return nil
}

func (m *MemDb) RollbackBatch() {
}

func (m *MemDb) GetLastRoot() (*big.Int, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.LastRoot, nil
}

func (m *MemDb) SetLastRoot(value *big.Int) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.LastRoot = value
	return nil
}

func (m *MemDb) Get(key utils.NodeKey) (utils.NodeValue12, error) {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])

	k := utils.ConvertBigIntToHex(keyConc)

	values := utils.NodeValue12{}
	for i, v := range m.Db[k] {
		values[i] = utils.ConvertHexToBigInt(v)
	}

	return values, nil
}

func (m *MemDb) Insert(key utils.NodeKey, value utils.NodeValue12) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])
	k := utils.ConvertBigIntToHex(keyConc)

	values := make([]string, 12)
	for i, v := range value {
		values[i] = utils.ConvertBigIntToHex(v)
	}

	m.Db[k] = values
	return nil
}

func (m *MemDb) GetAccountValue(key utils.NodeKey) (utils.NodeValue8, error) {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])

	k := utils.ConvertBigIntToHex(keyConc)

	values := utils.NodeValue8{}
	for i, v := range m.DbAccVal[k] {
		values[i] = utils.ConvertHexToBigInt(v)
	}

	return values, nil
}

func (m *MemDb) InsertAccountValue(key utils.NodeKey, value utils.NodeValue8) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done

	keyConc := utils.ArrayToScalar(key[:])
	k := utils.ConvertBigIntToHex(keyConc)

	values := make([]string, 8)
	for i, v := range value {
		values[i] = utils.ConvertBigIntToHex(v)
	}

	m.DbAccVal[k] = values
	return nil
}

func (m *MemDb) Delete(key string) error {
	m.lock.Lock()         // Lock for writing
	defer m.lock.Unlock() // Make sure to unlock when done

	delete(m.Db, key)
	return nil
}

func (m *MemDb) IsEmpty() bool {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	return len(m.Db) == 0
}

func (m *MemDb) PrintDb() {
	m.lock.RLock()         // Lock for reading
	defer m.lock.RUnlock() // Make sure to unlock when done

	for k, v := range m.Db {
		println(k, v)
	}
}

func (m *MemDb) GetDb() map[string][]string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.Db
}
