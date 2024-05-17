package commands

import (
	stages2 "github.com/tenderly/zkevm-erigon/turbo/stages"
	"github.com/tenderly/zkevm-erigon/eth/stagedsync"
	"github.com/gateway-fm/cdk-erigon-lib/common/datadir"
	"github.com/tenderly/zkevm-erigon/core"
	"github.com/gateway-fm/cdk-erigon-lib/kv"
	"github.com/c2h5oh/datasize"
	chain3 "github.com/gateway-fm/cdk-erigon-lib/chain"
	"github.com/gateway-fm/cdk-erigon-lib/kv/kvcfg"
	"github.com/tenderly/zkevm-erigon/cmd/hack/tool/fromdb"
	"github.com/tenderly/zkevm-erigon/turbo/shards"
	"github.com/tenderly/zkevm-erigon/cmd/sentry/sentry"
	"context"
	"github.com/tenderly/zkevm-erigon/core/vm"
	"github.com/tenderly/zkevm-erigon/consensus"
	"github.com/tenderly/zkevm-erigon/eth/ethconfig"
	stages3 "github.com/tenderly/zkevm-erigon/zk/stages"
	"github.com/tenderly/zkevm-erigon/zk/sequencer"
)

func newSyncZk(ctx context.Context, db kv.RwDB) (consensus.Engine, *vm.Config, *stagedsync.Sync) {
	historyV3, pm := kvcfg.HistoryV3.FromDB(db), fromdb.PruneMode(db)

	vmConfig := &vm.Config{}

	genesis := core.GenesisBlockByChainName(chain)
	chainConfig, genesisBlock, genesisErr := core.CommitGenesisBlock(db, genesis, "")
	if _, ok := genesisErr.(*chain3.ConfigCompatError); genesisErr != nil && !ok {
		panic(genesisErr)
	}
	//log.Info("Initialised chain configuration", "config", chainConfig)

	var batchSize datasize.ByteSize
	must(batchSize.UnmarshalText([]byte(batchSizeStr)))

	cfg := ethconfig.Defaults
	cfg.HistoryV3 = historyV3
	cfg.Prune = pm
	cfg.BatchSize = batchSize
	cfg.DeprecatedTxPool.Disable = true
	cfg.Genesis = core.GenesisBlockByChainName(chain)
	cfg.Dirs = datadir.New(datadirCli)

	engine := initConsensusEngine(chainConfig, cfg.Dirs.DataDir, db)

	br := getBlockReader(db)
	allSn, agg := allSnapshots(ctx, db)
	cfg.Snapshot = allSn.Cfg()

	sentryControlServer, err := sentry.NewMultiClient(
		db,
		"",
		chainConfig,
		genesisBlock.Hash(),
		engine,
		1,
		nil,
		ethconfig.Defaults.Sync,
		br,
		false,
		nil,
		ethconfig.Defaults.DropUselessPeers,
	)
	if err != nil {
		panic(err)
	}

	isSequencer := sequencer.IsSequencer()
	var stages []*stagedsync.Stage

	if isSequencer {
		stages = stages2.NewSequencerZkStages(
			context.Background(),
			db,
			&cfg,
			sentryControlServer,
			&shards.Notifications{},
			nil,
			allSn,
			agg,
			nil,
			engine,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
		)
	} else {
		stages = stages2.NewDefaultZkStages(
			context.Background(),
			db,
			&cfg,
			sentryControlServer,
			&shards.Notifications{},
			nil,
			allSn,
			agg,
			nil,
			engine,
			nil,
			nil,
			nil)
	}

	// set the unwind order depending on whether sequencer or synchronizer (ensure to set ENV VAR!)
	unwindOrder := stages3.ZkUnwindOrder
	if isSequencer {
		unwindOrder = stages3.ZkSequencerUnwindOrder
	}

	sync := stagedsync.New(stages, unwindOrder, stagedsync.DefaultPruneOrder)

	return engine, vmConfig, sync
}
