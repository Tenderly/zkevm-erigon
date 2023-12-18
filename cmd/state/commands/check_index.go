package commands

import (
	"github.com/spf13/cobra"
	"github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon/cmd/state/verify"
)

func init() {
	withDataDir(checkIndexCMD)
	withIndexBucket(checkIndexCMD)
	withCSBucket(checkIndexCMD)
	rootCmd.AddCommand(checkIndexCMD)
}

var checkIndexCMD = &cobra.Command{
	Use:   "checkIndex",
	Short: "Index checker",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, _ := common.RootContext()
		return verify.CheckIndex(ctx, chaindata, changeSetBucket, indexBucket)
	},
}
