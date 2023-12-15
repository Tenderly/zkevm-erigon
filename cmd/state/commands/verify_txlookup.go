package commands

import (
	"github.com/spf13/cobra"
	"github.com/tenderly/zkevm-erigon/cmd/state/verify"
)

func init() {
	withDataDir(verifyTxLookupCmd)
	rootCmd.AddCommand(verifyTxLookupCmd)
}

var verifyTxLookupCmd = &cobra.Command{
	Use:   "verifyTxLookup",
	Short: "Generate tx lookup index",
	RunE: func(cmd *cobra.Command, args []string) error {
		return verify.ValidateTxLookups(chaindata)
	},
}
