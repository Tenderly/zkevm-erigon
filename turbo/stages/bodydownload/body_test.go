package bodydownload

import (
	"testing"

	"github.com/gateway-fm/cdk-erigon-lib/kv/memdb"

	"github.com/tenderly/zkevm-erigon/consensus/ethash"
)

func TestCreateBodyDownload(t *testing.T) {
	_, tx := memdb.NewTestTx(t)
	bd := NewBodyDownload(ethash.NewFaker(), 100)
	if _, _, _, _, err := bd.UpdateFromDb(tx); err != nil {
		t.Fatalf("update from db: %v", err)
	}
}
