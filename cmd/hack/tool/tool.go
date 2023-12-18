package tool

import (
	"strconv"

	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon/chain"

	"github.com/tenderly/zkevm-erigon/core/rawdb"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func ParseFloat64(str string) float64 {
	v, _ := strconv.ParseFloat(str, 64)
	return v
}

func ChainConfig(tx kv.Tx) *chain.Config {
	genesisBlock, err := rawdb.ReadBlockByNumber(tx, 0)
	Check(err)
	chainConfig, err := rawdb.ReadChainConfig(tx, genesisBlock.Hash())
	Check(err)
	return chainConfig
}
