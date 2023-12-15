package commands

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tenderly/erigon/erigon-lib/common"
	"github.com/tenderly/erigon/erigon-lib/gointerfaces"
	"github.com/tenderly/erigon/erigon-lib/gointerfaces/remote"
)

// Test case for https://github.com/ethereum/execution-apis/pull/217 responses
func TestZeroLatestValidHash(t *testing.T) {
	payloadStatus := remote.EnginePayloadStatus{Status: remote.EngineStatus_INVALID, LatestValidHash: gointerfaces.ConvertHashToH256(common.Hash{})}
	json, err := convertPayloadStatus(context.TODO(), nil, &payloadStatus)
	require.NoError(t, err)
	assert.Equal(t, "INVALID", json["status"])
	assert.Equal(t, common.Hash{}, json["latestValidHash"])
}
