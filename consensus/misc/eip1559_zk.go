package misc

import (
	"math/big"
	"github.com/tenderly/zkevm-erigon/chain"
	"github.com/tenderly/zkevm-erigon/core/types"
)

func CalcBaseFeeZk(config *chain.Config, parent *types.Header) *big.Int {
	if config.SupportGasless {
		return big.NewInt(0)
	}

	return CalcBaseFee(config, parent)
}
