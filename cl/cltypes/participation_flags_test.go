package cltypes_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tenderly/zkevm-erigon/cl/cltypes"
)

func TestParticipationFlags(t *testing.T) {
	flagsList := cltypes.ParticipationFlagsListFromBytes([]byte{0, 0, 0, 0})
	flagsList[0] = flagsList[0].Add(4) // Turn on fourth bit
	require.True(t, flagsList[0].HasFlag(4))
}
