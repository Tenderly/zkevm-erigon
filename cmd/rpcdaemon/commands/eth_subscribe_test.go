package commands

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	libcommon "github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/direct"
	sentry "github.com/tenderly/zkevm-erigon-lib/gointerfaces/zkevm_sentry"

	"github.com/tenderly/zkevm-erigon/cmd/rpcdaemon/rpcservices"
	"github.com/tenderly/zkevm-erigon/core"
	"github.com/tenderly/zkevm-erigon/eth/protocols/eth"
	"github.com/tenderly/zkevm-erigon/ethdb/privateapi"
	"github.com/tenderly/zkevm-erigon/rlp"
	"github.com/tenderly/zkevm-erigon/turbo/rpchelper"
	"github.com/tenderly/zkevm-erigon/turbo/snapshotsync"
	"github.com/tenderly/zkevm-erigon/turbo/stages"
)

func TestEthSubscribe(t *testing.T) {
	m, require := stages.Mock(t), require.New(t)
	br := snapshotsync.NewBlockReaderWithSnapshots(m.BlockSnapshots, m.TransactionsV3)
	chain, err := core.GenerateChain(m.ChainConfig, m.Genesis, m.Engine, m.DB, 7, func(i int, b *core.BlockGen) {
		b.SetCoinbase(libcommon.Address{1})
	}, false /* intermediateHashes */)
	require.NoError(err)

	b, err := rlp.EncodeToBytes(&eth.BlockHeadersPacket66{
		RequestId:          1,
		BlockHeadersPacket: chain.Headers,
	})
	require.NoError(err)

	m.ReceiveWg.Add(1)
	for _, err = range m.Send(&sentry.InboundMessage{Id: sentry.MessageId_BLOCK_HEADERS_66, Data: b, PeerId: m.PeerId}) {
		require.NoError(err)
	}
	m.ReceiveWg.Wait() // Wait for all messages to be processed before we proceeed

	ctx := context.Background()
	backendServer := privateapi.NewEthBackendServer(ctx, nil, m.DB, m.Notifications.Events, br, nil, nil, nil, false)
	backendClient := direct.NewEthBackendClientDirect(backendServer)
	backend := rpcservices.NewRemoteBackend(backendClient, m.DB, br)
	ff := rpchelper.New(ctx, backend, nil, nil, func() {})

	newHeads, id := ff.SubscribeNewHeads(16)
	defer ff.UnsubscribeHeads(id)

	initialCycle := true
	highestSeenHeader := chain.TopBlock.NumberU64()
	if _, err := stages.StageLoopStep(m.Ctx, m.ChainConfig, m.DB, m.Sync, m.Notifications, initialCycle, m.UpdateHead); err != nil {
		t.Fatal(err)
	}

	for i := uint64(1); i <= highestSeenHeader; i++ {
		header := <-newHeads
		fmt.Printf("Got header %d\n", header.Number.Uint64())
		require.Equal(i, header.Number.Uint64())
	}
}
