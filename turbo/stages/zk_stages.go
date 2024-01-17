package stages

import (
	"context"

	"github.com/0xPolygonHermez/zkevm-data-streamer/datastreamer"
	proto_downloader "github.com/tenderly/erigon/erigon-lib/gointerfaces/downloader"
	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon-lib/state"
	"github.com/tenderly/zkevm-erigon/cmd/sentry/sentry"
	"github.com/tenderly/zkevm-erigon/consensus"
	"github.com/tenderly/zkevm-erigon/core/vm"
	"github.com/tenderly/zkevm-erigon/eth/ethconfig"
	"github.com/tenderly/zkevm-erigon/eth/stagedsync"
	"github.com/tenderly/zkevm-erigon/turbo/engineapi"
	"github.com/tenderly/zkevm-erigon/turbo/shards"
	"github.com/tenderly/zkevm-erigon/turbo/snapshotsync"
	"github.com/tenderly/zkevm-erigon/zk/datastream/client"
	zkStages "github.com/tenderly/zkevm-erigon/zk/stages"
	"github.com/tenderly/zkevm-erigon/zk/syncer"
)

// NewDefaultZkStages creates stages for zk syncer (RPC mode)
func NewDefaultZkStages(ctx context.Context,
	db kv.RwDB,
	cfg *ethconfig.Config,
	controlServer *sentry.MultiClient,
	notifications *shards.Notifications,
	snapDownloader proto_downloader.DownloaderClient,
	snapshots *snapshotsync.RoSnapshots,
	agg *state.AggregatorV3,
	forkValidator *engineapi.ForkValidator,
	engine consensus.Engine,
	l1Syncer *syncer.L1Syncer,
	datastreamClient *client.StreamClient,
	datastreamServer *datastreamer.StreamServer,
) []*stagedsync.Stage {
	dirs := cfg.Dirs
	blockReader := snapshotsync.NewBlockReaderWithSnapshots(snapshots, cfg.TransactionsV3)
	blockRetire := snapshotsync.NewBlockRetire(1, dirs.Tmp, snapshots, db, snapDownloader, notifications.Events)

	// During Import we don't want other services like header requests, body requests etc. to be running.
	// Hence we run it in the test mode.
	runInTestMode := cfg.ImportMode

	return zkStages.DefaultZkStages(ctx,
		zkStages.StageL1SyncerCfg(db, l1Syncer, cfg.Zk),
		zkStages.StageBatchesCfg(db, datastreamClient),
		zkStages.StageDataStreamCatchupCfg(datastreamServer, db),
		stagedsync.StageCumulativeIndexCfg(db),
		stagedsync.StageBlockHashesCfg(db, dirs.Tmp, controlServer.ChainConfig),
		stagedsync.StageSendersCfg(db, controlServer.ChainConfig, false, dirs.Tmp, cfg.Prune, blockRetire, controlServer.Hd),
		stagedsync.StageExecuteBlocksCfg(
			db,
			cfg.Prune,
			cfg.BatchSize,
			nil,
			controlServer.ChainConfig,
			controlServer.Engine,
			&vm.Config{},
			notifications.Accumulator,
			cfg.StateStream,
			/*stateStream=*/ false,
			cfg.HistoryV3,
			dirs,
			blockReader,
			controlServer.Hd,
			cfg.Genesis,
			cfg.Sync,
			agg,
			cfg.Zk,
		),
		stagedsync.StageHashStateCfg(db, dirs, cfg.HistoryV3, agg),
		zkStages.StageZkInterHashesCfg(db, true, true, false, dirs.Tmp, blockReader, controlServer.Hd, cfg.HistoryV3, agg, cfg.Zk),
		stagedsync.StageHistoryCfg(db, cfg.Prune, dirs.Tmp),
		stagedsync.StageLogIndexCfg(db, cfg.Prune, dirs.Tmp),
		stagedsync.StageCallTracesCfg(db, cfg.Prune, 0, dirs.Tmp),
		stagedsync.StageTxLookupCfg(db, cfg.Prune, dirs.Tmp, snapshots, controlServer.ChainConfig.Bor),
		stagedsync.StageFinishCfg(db, dirs.Tmp, forkValidator),
		runInTestMode)
}

// NewSequencerZkStages creates stages for a zk sequencer
func NewSequencerZkStages(ctx context.Context,
	db kv.RwDB,
	cfg *ethconfig.Config,
	controlServer *sentry.MultiClient,
	notifications *shards.Notifications,
	snapDownloader proto_downloader.DownloaderClient,
	snapshots *snapshotsync.RoSnapshots,
	agg *state.AggregatorV3,
	forkValidator *engineapi.ForkValidator,
	engine consensus.Engine,
	datastreamServer *datastreamer.StreamServer,
) []*stagedsync.Stage {
	dirs := cfg.Dirs
	blockReader := snapshotsync.NewBlockReaderWithSnapshots(snapshots, cfg.TransactionsV3)

	// During Import we don't want other services like header requests, body requests etc. to be running.
	// Hence we run it in the test mode.
	runInTestMode := cfg.ImportMode

	return zkStages.SequencerZkStages(ctx,
		stagedsync.StageCumulativeIndexCfg(db),
		zkStages.StageDataStreamCatchupCfg(datastreamServer, db),
		zkStages.StageSequenceBlocksCfg(
			db,
			cfg.Prune,
			cfg.BatchSize,
			nil,
			controlServer.ChainConfig,
			controlServer.Engine,
			&vm.Config{},
			notifications.Accumulator,
			cfg.StateStream,
			/*stateStream=*/ false,
			cfg.HistoryV3,
			dirs,
			blockReader,
			controlServer.Hd,
			cfg.Genesis,
			cfg.Sync,
			agg,
			cfg.Zk,
		),
		stagedsync.StageHashStateCfg(db, dirs, cfg.HistoryV3, agg),
		zkStages.StageZkInterHashesCfg(db, true, true, false, dirs.Tmp, blockReader, controlServer.Hd, cfg.HistoryV3, agg, cfg.Zk),
		stagedsync.StageHistoryCfg(db, cfg.Prune, dirs.Tmp),
		stagedsync.StageLogIndexCfg(db, cfg.Prune, dirs.Tmp),
		stagedsync.StageCallTracesCfg(db, cfg.Prune, 0, dirs.Tmp),
		stagedsync.StageTxLookupCfg(db, cfg.Prune, dirs.Tmp, snapshots, controlServer.ChainConfig.Bor),
		stagedsync.StageFinishCfg(db, dirs.Tmp, forkValidator),
		runInTestMode)
}
