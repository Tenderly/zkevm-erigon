package cltypes

import (
	libcommon "github.com/tenderly/erigon/erigon-lib/common"
	"github.com/tenderly/erigon/erigon-lib/common/length"
	"github.com/tenderly/zkevm-erigon/cl/cltypes/ssz"
	"github.com/tenderly/zkevm-erigon/cl/merkle_tree"
)

type HistoricalSummary struct {
	BlockSummaryRoot libcommon.Hash
	StateSummaryRoot libcommon.Hash
}

func (h *HistoricalSummary) EncodeSSZ(buf []byte) ([]byte, error) {
	return append(buf, append(h.BlockSummaryRoot[:], h.StateSummaryRoot[:]...)...), nil
}

func (h *HistoricalSummary) DecodeSSZ(buf []byte) error {
	if len(buf) < h.EncodingSizeSSZ() {
		return ssz.ErrLowBufferSize
	}
	copy(h.BlockSummaryRoot[:], buf)
	copy(h.StateSummaryRoot[:], buf[length.Hash:])
	return nil
}

func (h *HistoricalSummary) DecodeSSZWithVersion(buf []byte, _ int) error {
	return h.DecodeSSZ(buf)
}

func (h *HistoricalSummary) HashSSZ() ([32]byte, error) {
	return merkle_tree.ArraysRoot([][32]byte{h.BlockSummaryRoot, h.StateSummaryRoot}, 2)
}

func (*HistoricalSummary) EncodingSizeSSZ() int {
	return length.Hash * 2
}
