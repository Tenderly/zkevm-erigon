package stages

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon-lib/kv/memdb"
	"github.com/tenderly/zkevm-erigon/eth/stagedsync"
	"github.com/tenderly/zkevm-erigon/eth/stagedsync/stages"
	"github.com/tenderly/zkevm-erigon/smt/pkg/db"
	"github.com/tenderly/zkevm-erigon/zk/datastream/types"
	"github.com/tenderly/zkevm-erigon/zk/hermez_db"

	"github.com/stretchr/testify/require"
	"github.com/tenderly/zkevm-erigon/eth/ethconfig"
)

func TestUnwindBatches(t *testing.T) {
	fullL2Blocks := []types.FullL2Block{}
	post155 := "0xf86780843b9aca00826163941275fbb540c8efc58b812ba83b0d0b8b9917ae98808464fbb77c1ba0b7d2a666860f3c6b8f5ef96f86c7ec5562e97fd04c2e10f3755ff3a0456f9feba0246df95217bf9082f84f9e40adb0049c6664a5bb4c9cbe34ab1a73e77bab26ed"
	post155Bytes, err := hex.DecodeString(post155[2:])
	currentBlockNumber := 10

	require.NoError(t, err)
	for i := 1; i <= currentBlockNumber; i++ {
		fullL2Blocks = append(fullL2Blocks, types.FullL2Block{
			BatchNumber:     uint64(i / 2),
			L2BlockNumber:   uint64(i),
			Timestamp:       int64(i) * 10000,
			DeltaTimestamp:  uint32(i) * 10,
			L1InfoTreeIndex: uint32(i) + 20,
			GlobalExitRoot:  common.Hash{byte(i)},
			Coinbase:        common.Address{byte(i)},
			ForkId:          uint16(i) / 3,
			ChainId:         uint32(1),
			L1BlockHash:     common.Hash{byte(i)},
			L2Blockhash:     common.Hash{byte(i)},
			StateRoot:       common.Hash{byte(i)},
			L2Txs: []types.L2Transaction{
				{
					EffectiveGasPricePercentage: 255,
					IsValid:                     1,
					StateRoot:                   common.Hash{byte(i + 1)},
					EncodedLength:               uint32(len(post155Bytes)),
					Encoded:                     post155Bytes,
				},
			},
			ParentHash: common.Hash{byte(i - 1)},
		})
	}
	gerUpdates := []types.GerUpdate{}
	for i := currentBlockNumber + 1; i <= currentBlockNumber+5; i++ {
		gerUpdates = append(gerUpdates, types.GerUpdate{
			BatchNumber:    uint64(i / 2),
			Timestamp:      uint64(i) * 10000,
			GlobalExitRoot: common.Hash{byte(i)},
			Coinbase:       common.Address{byte(i)},
			ForkId:         uint16(i) / 3,
			ChainId:        uint32(1),
			StateRoot:      common.Hash{byte(i)},
		})
	}

	ctx, db1 := context.Background(), memdb.NewTestDB(t)
	tx := memdb.BeginRw(t, db1)
	err = hermez_db.CreateHermezBuckets(tx)
	require.NoError(t, err)

	err = db.CreateEriDbBuckets(tx)
	require.NoError(t, err)

	dsClient := NewTestDatastreamClient(fullL2Blocks, gerUpdates)
	cfg := StageBatchesCfg(db1, dsClient, &ethconfig.Zk{})

	s := &stagedsync.StageState{ID: stages.Batches, BlockNumber: 0}
	u := &stagedsync.Sync{}
	us := &stagedsync.UnwindState{ID: stages.Batches, UnwindPoint: 0, CurrentBlockNumber: uint64(currentBlockNumber)}
	err = stages.SaveStageProgress(tx, stages.L1VerificationsBatchNo, 20)
	require.NoError(t, err)

	// get bucket sizes pre inserts
	bucketSized := make(map[string]uint64)
	buckets, err := tx.ListBuckets()
	require.NoError(t, err)
	for _, bucket := range buckets {
		size, err := tx.BucketSize(bucket)
		require.NoError(t, err)
		bucketSized[bucket] = size
	}

	/////////
	// ACT //
	/////////
	err = SpawnStageBatches(s, u, ctx, tx, cfg, true, true)
	require.NoError(t, err)

	tx2 := memdb.BeginRw(t, db1)

	// unwind to zero and check if there is any data in the tables
	err = UnwindBatchesStage(us, tx2, cfg, ctx)
	require.NoError(t, err)
	tx2.Commit()

	////////////////
	// ASSERTIONS //
	////////////////
	// check if there is any data in the tables
	tx3 := memdb.BeginRw(t, db1)
	buckets, err = tx3.ListBuckets()
	require.NoError(t, err)
	for _, bucket := range buckets {
		//currently not decrementing sequence
		if bucket == kv.Sequence {
			continue
		}
		size, err := tx3.BucketSize(bucket)
		require.NoError(t, err)
		require.Equal(t, bucketSized[bucket], size, "butcket %s is not empty", bucket)
	}
}
