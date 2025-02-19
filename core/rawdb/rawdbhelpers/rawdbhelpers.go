package rawdbhelpers

import (
	"encoding/binary"

	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon/eth/ethconfig"
)

func IdxStepsCountV3(tx kv.Tx) float64 {
	fst, _ := kv.FirstKey(tx, kv.TracesToKeys)
	lst, _ := kv.LastKey(tx, kv.TracesToKeys)
	if len(fst) > 0 && len(lst) > 0 {
		fstTxNum := binary.BigEndian.Uint64(fst)
		lstTxNum := binary.BigEndian.Uint64(lst)

		return float64(lstTxNum-fstTxNum) / float64(ethconfig.HistoryV3AggregationStep)
	}
	return 0
}
