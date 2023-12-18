package state

import (
	"time"

	"github.com/tenderly/zkevm-erigon-lib/common"
)

// GlobalExitRoot struct
type GlobalExitRoot struct {
	BlockNumber     uint64
	Timestamp       time.Time
	MainnetExitRoot common.Hash
	RollupExitRoot  common.Hash
	GlobalExitRoot  common.Hash
}

type GlobalExitRootDb struct {
	GlobalExitRoot         common.Hash
	GlobalExitRootPosition common.Hash
	Timestamp              int64
}
