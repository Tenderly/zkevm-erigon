package commands

import (
	types "github.com/tenderly/zkevm-erigon/zk/rpcdaemon"
	"github.com/gateway-fm/cdk-erigon-lib/common"
)

type ZkExitRoots struct {
	BlockNumber     types.ArgUint64 `json:"blockNumber"`
	Timestamp       types.ArgUint64 `json:"timestamp"`
	MainnetExitRoot common.Hash     `json:"mainnetExitRoot"`
	RollupExitRoot  common.Hash     `json:"rollupExitRoot"`
}
