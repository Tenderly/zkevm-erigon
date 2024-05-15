package commands

import (
	"fmt"

	"github.com/tenderly/zkevm-erigon/cmd/devnet/models"
	"github.com/tenderly/zkevm-erigon/cmd/devnet/requests"
)

func pingErigonRpc() error {
	err := requests.PingErigonRpc(models.ReqId)
	if err != nil {
		fmt.Printf("FAILURE => %v\n", err)
	}
	return err
}
