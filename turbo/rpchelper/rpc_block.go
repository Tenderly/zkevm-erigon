package rpchelper

import (
	"fmt"

	libcommon "github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/kv"

	"github.com/tenderly/zkevm-erigon/core/rawdb"
	"github.com/tenderly/zkevm-erigon/rpc"
	"github.com/tenderly/zkevm-erigon/sync_stages"
)

var UnknownBlockError = &rpc.CustomError{
	Code:    -39001,
	Message: "Unknown block",
}

func GetLatestBlockNumber(tx kv.Tx) (uint64, error) {
	forkchoiceHeadHash := rawdb.ReadForkchoiceHead(tx)
	if forkchoiceHeadHash != (libcommon.Hash{}) {
		forkchoiceHeadNum := rawdb.ReadHeaderNumber(tx, forkchoiceHeadHash)
		if forkchoiceHeadNum != nil {
			return *forkchoiceHeadNum, nil
		}
	}

	blockNum, err := sync_stages.GetStageProgress(tx, sync_stages.Execution)
	if err != nil {
		return 0, fmt.Errorf("getting latest block number: %w", err)
	}

	return blockNum, nil
}

func GetFinalizedBlockNumber(tx kv.Tx) (uint64, error) {
	forkchoiceFinalizedHash := rawdb.ReadForkchoiceFinalized(tx)
	if forkchoiceFinalizedHash != (libcommon.Hash{}) {
		forkchoiceFinalizedNum := rawdb.ReadHeaderNumber(tx, forkchoiceFinalizedHash)
		if forkchoiceFinalizedNum != nil {
			return *forkchoiceFinalizedNum, nil
		}
	}

	return 0, UnknownBlockError
}

func GetSafeBlockNumber(tx kv.Tx) (uint64, error) {
	forkchoiceSafeHash := rawdb.ReadForkchoiceSafe(tx)
	if forkchoiceSafeHash != (libcommon.Hash{}) {
		forkchoiceSafeNum := rawdb.ReadHeaderNumber(tx, forkchoiceSafeHash)
		if forkchoiceSafeNum != nil {
			return *forkchoiceSafeNum, nil
		}
	}
	return 0, UnknownBlockError
}

func GetLatestExecutedBlockNumber(tx kv.Tx) (uint64, error) {
	blockNum, err := sync_stages.GetStageProgress(tx, sync_stages.Execution)
	if err != nil {
		return 0, err
	}
	return blockNum, err
}
