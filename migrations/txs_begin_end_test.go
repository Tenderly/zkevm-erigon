package migrations

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/tenderly/zkevm-erigon/eth/stagedsync/stages"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	libcommon "github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/common/hexutility"
	"github.com/tenderly/zkevm-erigon-lib/common/u256"
	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon-lib/kv/memdb"

	"github.com/tenderly/zkevm-erigon/core/rawdb"
	"github.com/tenderly/zkevm-erigon/core/types"
)

func TestTxsBeginEnd(t *testing.T) {
	require, tmpDir, db := require.New(t), t.TempDir(), memdb.NewTestDB(t)
	txn := &types.DynamicFeeTransaction{Tip: u256.N1, FeeCap: u256.N1, CommonTx: types.CommonTx{ChainID: u256.N1, Value: u256.N1, Gas: 1, Nonce: 1}}
	buf := bytes.NewBuffer(nil)
	err := txn.MarshalBinary(buf)
	require.NoError(err)
	rlpTxn := buf.Bytes()
	logEvery := time.NewTicker(10 * time.Second)
	defer logEvery.Stop()

	b := &types.RawBody{Transactions: [][]byte{rlpTxn, rlpTxn, rlpTxn}}
	err = db.Update(context.Background(), func(tx kv.RwTx) error {
		for i := uint64(0); i < 10; i++ {
			hash := libcommon.Hash{byte(i)}
			err = writeRawBodyDeprecated(tx, hash, i, b)
			require.NoError(err)
			err = rawdb.WriteCanonicalHash(tx, hash, i)
			require.NoError(err)
		}
		if err := makeBodiesNonCanonicalDeprecated(tx, 7, context.Background(), "", logEvery); err != nil {
			return err
		}

		err = rawdb.TruncateCanonicalHash(tx, 7, false)
		for i := uint64(7); i < 10; i++ {
			require.NoError(err)
			hash := libcommon.Hash{0xa, byte(i)}
			err = writeRawBodyDeprecated(tx, hash, i, b)
			require.NoError(err)
			err = rawdb.WriteCanonicalHash(tx, hash, i)
			require.NoError(err)
		}
		if err := stages.SaveStageProgress(tx, stages.Bodies, 9); err != nil {
			return err
		}
		return nil
	})
	require.NoError(err)

	migrator := NewMigrator(kv.ChainDB)
	migrator.Migrations = []Migration{txsBeginEnd}
	err = migrator.Apply(db, tmpDir)
	require.NoError(err)

	err = db.View(context.Background(), func(tx kv.Tx) error {
		v, err := tx.ReadSequence(kv.EthTx)
		require.NoError(err)
		require.Equal(uint64(3*10+2*10), v)
		return nil
	})
	require.NoError(err)

	err = db.View(context.Background(), func(tx kv.Tx) error {
		for i := uint64(7); i < 10; i++ {
			hash := libcommon.Hash{byte(i)}
			k := make([]byte, 8+32)
			binary.BigEndian.PutUint64(k, 7)
			copy(k[8:], hash[:])

			has, err := tx.Has(kv.BlockBody, k)
			require.NoError(err)
			require.False(has)
		}

		c, err := tx.Cursor(kv.NonCanonicalTxs)
		require.NoError(err)
		cnt, err := c.Count()
		require.NoError(err)
		require.Zero(cnt)

		v, err := tx.ReadSequence(kv.NonCanonicalTxs)
		require.NoError(err)
		require.Zero(v)

		has, err := tx.Has(kv.EthTx, hexutility.EncodeTs(0))
		require.NoError(err)
		require.False(has)

		return nil
	})
	require.NoError(err)

}
