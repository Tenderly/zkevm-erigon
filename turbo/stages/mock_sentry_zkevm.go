package stages

import (
	"github.com/gateway-fm/cdk-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon/zk/hermez_db"
)

func withHermezDb(tx kv.RwTx) error {
	return hermez_db.CreateHermezBuckets(tx)
}
