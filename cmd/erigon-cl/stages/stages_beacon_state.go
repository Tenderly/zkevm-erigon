package stages

import (
	"context"
	"github.com/tenderly/zkevm-erigon/eth/stagedsync/stages"

	"github.com/ledgerwatch/log/v3"
	libcommon "github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon/cl/clparams"
	"github.com/tenderly/zkevm-erigon/cl/utils"
	"github.com/tenderly/zkevm-erigon/cmd/erigon-cl/core/rawdb"
	"github.com/tenderly/zkevm-erigon/cmd/erigon-cl/core/state"
	"github.com/tenderly/zkevm-erigon/cmd/erigon-cl/core/transition"
	"github.com/tenderly/zkevm-erigon/cmd/erigon-cl/execution_client"
)

type StageBeaconStateCfg struct {
	db              kv.RwDB
	beaconCfg       *clparams.BeaconChainConfig
	state           *state.BeaconState
	executionClient *execution_client.ExecutionClient
}

func StageBeaconState(db kv.RwDB,
	beaconCfg *clparams.BeaconChainConfig, state *state.BeaconState, executionClient *execution_client.ExecutionClient) StageBeaconStateCfg {
	return StageBeaconStateCfg{
		db:              db,
		beaconCfg:       beaconCfg,
		state:           state,
		executionClient: executionClient,
	}
}

// SpawnStageBeaconForward spawn the beacon forward stage
func SpawnStageBeaconState(cfg StageBeaconStateCfg, tx kv.RwTx, ctx context.Context) error {
	useExternalTx := tx != nil
	var err error
	if !useExternalTx {
		tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	endSlot, err := stages.GetStageProgress(tx, stages.BeaconBlocks)
	if err != nil {
		return err
	}
	latestBlockHeader := cfg.state.LatestBlockHeader()

	fromSlot := latestBlockHeader.Slot
	for slot := fromSlot + 1; slot <= endSlot; slot++ {
		finalizedRoot, err := rawdb.ReadFinalizedBlockRoot(tx, slot)
		if err != nil {
			return err
		}
		// Slot had a missing proposal in this case.
		if finalizedRoot == (libcommon.Hash{}) {
			continue
		}
		block, eth1Number, eth1Hash, err := rawdb.ReadBeaconBlock(tx, finalizedRoot, slot)
		if err != nil {
			return err
		}

		// Query execution engine only if the payload have an hash.
		if eth1Hash != (libcommon.Hash{}) {
			if block.Block.Body.ExecutionPayload, err = cfg.executionClient.ReadExecutionPayload(eth1Number, eth1Hash); err != nil {
				return err
			}
		}
		// validate fully only in current epoch.
		fullValidate := utils.GetCurrentEpoch(cfg.state.GenesisTime(), cfg.beaconCfg.SecondsPerSlot, cfg.beaconCfg.SlotsPerEpoch) == cfg.state.Epoch()
		if err := transition.TransitionState(cfg.state, block, fullValidate); err != nil {
			log.Info("Found epoch, so stopping now...", "count", slot-(fromSlot+1), "slot", slot)
			return err
		}
		log.Info("Applied state transition", "from", slot, "to", slot+1)
	}
	// If successful update fork choice
	finalizedRoot, err := rawdb.ReadFinalizedBlockRoot(tx, endSlot)
	if err != nil {
		return err
	}
	_, _, eth1Hash, _, err := rawdb.ReadBeaconBlockForStorage(tx, finalizedRoot, endSlot)
	if err != nil {
		return err
	}
	receipt, err := cfg.executionClient.ForkChoiceUpdate(eth1Hash)
	if err != nil {
		return err
	}
	log.Info("Forkchoice Status", "outcome", receipt.Success)

	log.Info("[BeaconState] Finished transitioning state", "from", fromSlot, "to", endSlot)
	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
