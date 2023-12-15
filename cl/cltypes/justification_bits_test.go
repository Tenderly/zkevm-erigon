package cltypes_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tenderly/zkevm-erigon/cl/cltypes"
)

func TestParticipationBits(t *testing.T) {
	bits := cltypes.JustificationBits{}
	bits.FromByte(2)
	require.Equal(t, bits, cltypes.JustificationBits{false, true, false, false})
	require.Equal(t, bits.Byte(), byte(2))
}
