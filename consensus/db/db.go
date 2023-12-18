package db

import (
	"github.com/ledgerwatch/log/v3"
	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon-lib/kv/mdbx"
)

func OpenDatabase(path string, inMem bool, readonly bool) kv.RwDB {
	opts := mdbx.NewMDBX(log.Root()).Label(kv.ConsensusDB)
	if readonly {
		opts = opts.Readonly()
	}
	if inMem {
		opts = opts.InMem("")
	} else {
		opts = opts.Path(path)
	}

	return opts.MustOpen()
}
