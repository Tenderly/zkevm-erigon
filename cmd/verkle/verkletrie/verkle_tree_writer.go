package verkletrie

import (
	"encoding/binary"
	"github.com/anacrolix/sync"
	libcommon "github.com/gateway-fm/cdk-erigon-lib/common"
	"github.com/gateway-fm/cdk-erigon-lib/etl"
	"github.com/gateway-fm/cdk-erigon-lib/kv"
	"github.com/holiman/uint256"

	"github.com/tenderly/zkevm-erigon/core/types/accounts"
	"github.com/tenderly/zkevm-erigon/turbo/trie/vtree"
)

func int256ToVerkleFormat(x *uint256.Int, buffer []byte) {
	bbytes := x.ToBig().Bytes()
	if len(bbytes) > 0 {
		for i, b := range bbytes {
			buffer[len(bbytes)-i-1] = b
		}
	}
}

type VerkleTreeWriter struct {
	db        kv.RwTx
	collector *etl.Collector
	mu        sync.Mutex
	tmpdir    string
}

func NewVerkleTreeWriter(db kv.RwTx, tmpdir string) *VerkleTreeWriter {
	return &VerkleTreeWriter{
		db:        db,
		collector: etl.NewCollector("verkleTreeWriterLogPrefix", tmpdir, etl.NewSortableBuffer(etl.BufferOptimalSize*8)),
		tmpdir:    tmpdir,
	}
}

func (v *VerkleTreeWriter) UpdateAccount(versionKey []byte, codeSize uint64, isContract bool, acc accounts.Account) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	var codeHashKey, nonceKey, balanceKey, codeSizeKey, nonce, balance, cs [32]byte
	copy(codeHashKey[:], versionKey[:31])
	copy(nonceKey[:], versionKey[:31])
	copy(balanceKey[:], versionKey[:31])
	copy(codeSizeKey[:], versionKey[:31])
	codeHashKey[31] = vtree.CodeKeccakLeafKey
	nonceKey[31] = vtree.NonceLeafKey
	balanceKey[31] = vtree.BalanceLeafKey
	codeSizeKey[31] = vtree.CodeSizeLeafKey
	// Process values
	int256ToVerkleFormat(&acc.Balance, balance[:])
	binary.LittleEndian.PutUint64(nonce[:], acc.Nonce)

	// Insert in the tree
	if err := v.collector.Collect(versionKey, []byte{0}); err != nil {
		return err
	}

	if err := v.collector.Collect(nonceKey[:], nonce[:]); err != nil {
		return err
	}
	if err := v.collector.Collect(balanceKey[:], balance[:]); err != nil {
		return err
	}
	if isContract {
		binary.LittleEndian.PutUint64(cs[:], codeSize)
		if err := v.collector.Collect(codeHashKey[:], acc.CodeHash[:]); err != nil {
			return err
		}
		if err := v.collector.Collect(codeSizeKey[:], cs[:]); err != nil {
			return err
		}
	}
	return nil
}

func (v *VerkleTreeWriter) DeleteAccount(versionKey []byte, isContract bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	var codeHashKey, nonceKey, balanceKey, codeSizeKey [32]byte
	copy(codeHashKey[:], versionKey[:31])
	copy(nonceKey[:], versionKey[:31])
	copy(balanceKey[:], versionKey[:31])
	copy(codeSizeKey[:], versionKey[:31])
	codeHashKey[31] = vtree.CodeKeccakLeafKey
	nonceKey[31] = vtree.NonceLeafKey
	balanceKey[31] = vtree.BalanceLeafKey
	codeSizeKey[31] = vtree.CodeSizeLeafKey
	// Insert in the tree
	if err := v.collector.Collect(versionKey, []byte{0}); err != nil {
		return err
	}

	if err := v.collector.Collect(nonceKey[:], []byte{0}); err != nil {
		return err
	}
	if err := v.collector.Collect(balanceKey[:], []byte{0}); err != nil {
		return err
	}
	if isContract {
		if err := v.collector.Collect(codeHashKey[:], []byte{0}); err != nil {
			return err
		}
		if err := v.collector.Collect(codeSizeKey[:], []byte{0}); err != nil {
			return err
		}
	}
	return nil
}

func (v *VerkleTreeWriter) Insert(key, value []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.collector.Collect(key, value)
}

func (v *VerkleTreeWriter) WriteContractCodeChunks(codeKeys [][]byte, chunks [][]byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	for i, codeKey := range codeKeys {
		if err := v.collector.Collect(codeKey, chunks[i]); err != nil {
			return err
		}
	}
	return nil
}

func (v *VerkleTreeWriter) CommitVerkleTreeFromScratch() (libcommon.Hash, error) {
	return libcommon.Hash{}, nil
}

func (v *VerkleTreeWriter) CommitVerkleTree(root libcommon.Hash) (libcommon.Hash, error) {
	return libcommon.Hash{}, nil
}

func (v *VerkleTreeWriter) Close() {
	v.collector.Close()
}
