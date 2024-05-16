package services

import (
	"fmt"

	libcommon "github.com/gateway-fm/cdk-erigon-lib/common"

	"github.com/tenderly/zkevm-erigon/cmd/devnet/models"
	"github.com/tenderly/zkevm-erigon/cmd/devnet/requests"
)

func GetNonce(reqId int, address libcommon.Address) (uint64, error) {
	res, err := requests.GetTransactionCount(reqId, address, models.Latest)
	if err != nil {
		return 0, fmt.Errorf("failed to get transaction count for address 0x%x: %v", address, err)
	}

	return uint64(res.Result), nil
}
