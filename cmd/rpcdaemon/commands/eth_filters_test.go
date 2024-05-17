//go:build notzkevm
// +build notzkevm

package commands

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	libcommon "github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/common/length"

	"github.com/tenderly/zkevm-erigon/rpc/rpccfg"

	"github.com/tenderly/zkevm-erigon-lib/gointerfaces/txpool"
	"github.com/tenderly/zkevm-erigon-lib/kv/kvcache"
	"github.com/stretchr/testify/assert"

	"github.com/tenderly/zkevm-erigon/cmd/rpcdaemon/rpcdaemontest"
	"github.com/tenderly/zkevm-erigon/eth/ethconfig"
	"github.com/tenderly/zkevm-erigon/eth/filters"
	"github.com/tenderly/zkevm-erigon/turbo/rpchelper"
	"github.com/tenderly/zkevm-erigon/turbo/snapshotsync"
	"github.com/tenderly/zkevm-erigon/turbo/stages"
)

func TestNewFilters(t *testing.T) {
	assert := assert.New(t)
	m, _, _ := rpcdaemontest.CreateTestSentry(t)
	agg := m.HistoryV3Components()
	br := snapshotsync.NewBlockReaderWithSnapshots(m.BlockSnapshots, m.TransactionsV3)
	stateCache := kvcache.New(kvcache.DefaultCoherentConfig)
	ctx, conn := rpcdaemontest.CreateTestGrpcConn(t, stages.Mock(t))
	mining := txpool.NewMiningClient(conn)
	ff := rpchelper.New(ctx, nil, nil, mining, func() {})
	api := NewEthAPI(NewBaseApi(ff, stateCache, br, agg, false, rpccfg.DefaultEvmCallTimeout, m.Engine, m.Dirs), m.DB, nil, nil, nil, 5000000, 100_000, ethconfig.DefaultZkConfig)

	ptf, err := api.NewPendingTransactionFilter(ctx)
	assert.Nil(err)

	nf, err := api.NewFilter(ctx, filters.FilterCriteria{})
	assert.Nil(err)

	bf, err := api.NewBlockFilter(ctx)
	assert.Nil(err)

	ok, err := api.UninstallFilter(ctx, nf)
	assert.Nil(err)
	assert.Equal(ok, true)

	ok, err = api.UninstallFilter(ctx, bf)
	assert.Nil(err)
	assert.Equal(ok, true)

	ok, err = api.UninstallFilter(ctx, ptf)
	assert.Nil(err)
	assert.Equal(ok, true)
}

func TestLogsSubscribeAndUnsubscribe_WithoutConcurrentMapIssue(t *testing.T) {
	ctx, conn := rpcdaemontest.CreateTestGrpcConn(t, stages.Mock(t))
	mining := txpool.NewMiningClient(conn)
	ff := rpchelper.New(ctx, nil, nil, mining, func() {})

	// generate some random topics
	topics := make([][]libcommon.Hash, 0)
	for i := 0; i < 10; i++ {
		bytes := make([]byte, length.Hash)
		rand.Read(bytes)
		toAdd := []libcommon.Hash{libcommon.BytesToHash(bytes)}
		topics = append(topics, toAdd)
	}

	// generate some addresses
	addresses := make([]libcommon.Address, 0)
	for i := 0; i < 10; i++ {
		bytes := make([]byte, length.Addr)
		rand.Read(bytes)
		addresses = append(addresses, libcommon.BytesToAddress(bytes))
	}

	crit := filters.FilterCriteria{
		Topics:    topics,
		Addresses: addresses,
	}

	ids := make([]rpchelper.LogsSubID, 1000)

	// make a lot of subscriptions
	wg := sync.WaitGroup{}
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(idx int) {
			_, id := ff.SubscribeLogs(32, crit)
			defer func() {
				time.Sleep(100 * time.Nanosecond)
				ff.UnsubscribeLogs(id)
				wg.Done()
			}()
			ids[idx] = id
		}(i)
	}
	wg.Wait()
}
