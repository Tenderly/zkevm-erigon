package stages

import (
	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon/zk/hermez_db"
)

func withHermezDb(tx kv.RwTx) error {
	return hermez_db.CreateHermezBuckets(tx)
}
