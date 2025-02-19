package rawdb_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tenderly/zkevm-erigon-lib/kv/memdb"
	"github.com/tenderly/zkevm-erigon/cl/cltypes"
	"github.com/tenderly/zkevm-erigon/cmd/erigon-cl/core/rawdb"
)

func TestBytes2(t *testing.T) {
	l := 1000
	buf := rawdb.Bytes2FromLength(l)
	require.Equal(t, l, rawdb.LengthFromBytes2(buf))
}

var emptyBlock = &cltypes.Eth1Block{}

func TestBeaconBlock(t *testing.T) {
	_, tx := memdb.NewTestTx(t)
	signedBeaconBlock := &cltypes.SignedBeaconBlock{
		Block: &cltypes.BeaconBlock{
			Body: &cltypes.BeaconBody{
				Eth1Data:         &cltypes.Eth1Data{},
				Graffiti:         make([]byte, 32),
				SyncAggregate:    &cltypes.SyncAggregate{},
				ExecutionPayload: emptyBlock,
			},
		},
	}

	root, err := signedBeaconBlock.Block.HashSSZ()
	require.NoError(t, err)

	require.NoError(t, rawdb.WriteBeaconBlock(tx, signedBeaconBlock))
	newBlock, _, _, err := rawdb.ReadBeaconBlock(tx, root, signedBeaconBlock.Block.Slot)
	require.NoError(t, err)
	newBlock.Block.Body.ExecutionPayload = emptyBlock
	newRoot, err := newBlock.HashSSZ()
	require.NoError(t, err)
	signedBeaconBlock.Block.Body.ExecutionPayload = emptyBlock
	root, err = signedBeaconBlock.HashSSZ()
	require.NoError(t, err)

	require.Equal(t, root, newRoot)
}
