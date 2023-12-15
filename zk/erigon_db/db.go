package erigon_db

import (
	"fmt"
	"math/big"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"

	"github.com/tenderly/zkevm-erigon/core/rawdb"
	ethTypes "github.com/tenderly/zkevm-erigon/core/types"
)

var sha3UncleHash = common.HexToHash("0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347")

type ErigonDb struct {
	tx kv.RwTx
}

func NewErigonDb(tx kv.RwTx) *ErigonDb {
	return &ErigonDb{
		tx: tx,
	}
}

func (db ErigonDb) WriteHeader(
	blockNo *big.Int,
	stateRoot, txHash, parentHash common.Hash,
	coinbase common.Address,
	ts uint64,
) (*ethTypes.Header, error) {
	h := &ethTypes.Header{
		ParentHash: parentHash,
		UncleHash:  sha3UncleHash,
		Coinbase:   coinbase,
		Root:       stateRoot,
		TxHash:     txHash,
		Difficulty: big.NewInt(0),
		Number:     blockNo,
		GasLimit:   30_000_000,
		Time:       ts,
		Extra:      make([]byte, 0),
	}

	rawdb.WriteHeader(db.tx, h)
	err := rawdb.WriteCanonicalHash(db.tx, h.Hash(), blockNo.Uint64())
	if err != nil {
		return nil, fmt.Errorf("failed to write canonical hash: %w", err)
	}
	return h, nil
}

func (db ErigonDb) DeleteHeaders(blockFrom uint64) error {
	return rawdb.TruncateCanonicalHash(db.tx, blockFrom, true)
}

func (db ErigonDb) WriteBody(batchNo *big.Int, headerHash common.Hash, txs []ethTypes.Transaction) error {
	b := &ethTypes.Body{
		Transactions: txs,
	}

	// writes txs to EthTx (canonical table)
	return rawdb.WriteBody(db.tx, headerHash, batchNo.Uint64(), b)
}

func (db ErigonDb) DeleteBodies(blockFrom uint64) error {
	return rawdb.TruncateBodies(db.tx, blockFrom)
}

func (db ErigonDb) ReadCanonicalHash(blockNo uint64) (common.Hash, error) {
	return rawdb.ReadCanonicalHash(db.tx, blockNo)
}

func (db ErigonDb) GetHeader(blockNo uint64) (*ethTypes.Header, error) {
	hash, err := db.ReadCanonicalHash(blockNo)
	if err != nil {
		return nil, fmt.Errorf("failed to read canonical hash: %w", err)
	}
	return rawdb.ReadHeader(db.tx, hash, blockNo), nil
}
