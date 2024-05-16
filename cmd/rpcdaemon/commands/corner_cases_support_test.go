//go:build notzkevm
// +build notzkevm

package commands

import (
	"context"
	"testing"

	"github.com/gateway-fm/cdk-erigon-lib/common"
	"github.com/gateway-fm/cdk-erigon-lib/kv/kvcache"
	"github.com/stretchr/testify/require"

	"github.com/tenderly/zkevm-erigon/cmd/rpcdaemon/rpcdaemontest"
	"github.com/tenderly/zkevm-erigon/eth/ethconfig"
	"github.com/tenderly/zkevm-erigon/rpc"
	"github.com/tenderly/zkevm-erigon/rpc/rpccfg"
	"github.com/tenderly/zkevm-erigon/turbo/snapshotsync"
)

// TestNotFoundMustReturnNil - next methods - when record not found in db - must return nil instead of error
// see https://github.com/tenderly/zkevm-erigon/issues/1645
func TestNotFoundMustReturnNil(t *testing.T) {
	require := require.New(t)
	m, _, _ := rpcdaemontest.CreateTestSentry(t)
	agg := m.HistoryV3Components()
	br := snapshotsync.NewBlockReaderWithSnapshots(m.BlockSnapshots, m.TransactionsV3)
	stateCache := kvcache.New(kvcache.DefaultCoherentConfig)
	api := NewEthAPI(
		NewBaseApi(nil, stateCache, br, agg, false, rpccfg.DefaultEvmCallTimeout, m.Engine, m.Dirs),
		m.DB, nil, nil, nil, 5000000, 100_000, ethconfig.DefaultZkConfig)
	ctx := context.Background()

	a, err := api.GetTransactionByBlockNumberAndIndex(ctx, 10_000, 1)
	require.Nil(a)
	require.Nil(err)

	b, err := api.GetTransactionByBlockHashAndIndex(ctx, common.Hash{}, 1)
	require.Nil(b)
	require.Nil(err)

	c, err := api.GetTransactionByBlockNumberAndIndex(ctx, 10_000, 1)
	require.Nil(c)
	require.Nil(err)

	d, err := api.GetTransactionReceipt(ctx, common.Hash{})
	require.Nil(d)
	require.Nil(err)

	e, err := api.GetBlockByHash(ctx, rpc.BlockNumberOrHashWithHash(common.Hash{}, true), false)
	require.Nil(e)
	require.Nil(err)

	f, err := api.GetBlockByNumber(ctx, 10_000, false)
	require.Nil(f)
	require.Nil(err)

	g, err := api.GetUncleByBlockHashAndIndex(ctx, common.Hash{}, 1)
	require.Nil(g)
	require.Nil(err)

	h, err := api.GetUncleByBlockNumberAndIndex(ctx, 10_000, 1)
	require.Nil(h)
	require.Nil(err)

	j, err := api.GetBlockTransactionCountByNumber(ctx, 10_000)
	require.Nil(j)
	require.Nil(err)
}
