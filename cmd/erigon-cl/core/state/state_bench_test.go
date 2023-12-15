package state_test

import (
	"testing"

	eth2_shuffle "github.com/protolambda/eth2-shuffle"
	"github.com/tenderly/zkevm-erigon/cl/clparams"
	"github.com/tenderly/zkevm-erigon/cl/utils"
	"github.com/tenderly/zkevm-erigon/cmd/erigon-cl/core/state"
)

func BenchmarkLambdaShuffledIndex(b *testing.B) {
	keccakOptimized := utils.OptimizedKeccak256()
	eth2ShuffleHash := func(data []byte) []byte {
		hashed := keccakOptimized(data)
		return hashed[:]
	}
	seed := [32]byte{2, 35, 6}
	for i := 0; i < b.N; i++ {
		eth2_shuffle.PermuteIndex(eth2ShuffleHash, uint8(clparams.MainnetBeaconConfig.ShuffleRoundCount), 10, 1000, seed)
	}
}

// Faster by ~40%, the effects of it will be felt mostly on computation of the proposer index.
func BenchmarkErigonShuffledIndex(b *testing.B) {
	s := state.GetEmptyBeaconState()
	keccakOptimized := utils.OptimizedKeccak256()

	seed := [32]byte{2, 35, 6}
	preInputs := s.ComputeShuffledIndexPreInputs(seed)
	for i := 0; i < b.N; i++ {
		s.ComputeShuffledIndex(10, 1000, seed, preInputs, keccakOptimized)
	}
}
