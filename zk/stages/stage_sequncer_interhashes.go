package stages

import (
	"context"
	"fmt"
	"github.com/ledgerwatch/log/v3"
	libcommon "github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon/common/dbutils"
	"github.com/tenderly/zkevm-erigon/core/rawdb"
	state2 "github.com/tenderly/zkevm-erigon/core/state"
	"github.com/tenderly/zkevm-erigon/core/types"
	"github.com/tenderly/zkevm-erigon/core/types/accounts"
	"github.com/tenderly/zkevm-erigon/eth/stagedsync"
	db2 "github.com/tenderly/zkevm-erigon/smt/pkg/db"
	"github.com/tenderly/zkevm-erigon/smt/pkg/smt"
	"github.com/tenderly/zkevm-erigon/smt/pkg/utils"
	"github.com/tenderly/zkevm-erigon/turbo/trie"
	"github.com/tenderly/zkevm-erigon/zk/erigon_db"
	"time"
)

type SequencerInterhashesCfg struct {
	db kv.RwDB
}

func StageSequencerInterhashesCfg(db kv.RwDB) SequencerInterhashesCfg {
	return SequencerInterhashesCfg{
		db: db,
	}
}

func SpawnSequencerInterhashesStage(
	s *stagedsync.StageState,
	u stagedsync.Unwinder,
	tx kv.RwTx,
	ctx context.Context,
	cfg SequencerInterhashesCfg,
	initialCycle bool,
	quiet bool,
) error {
	var err error
	freshTx := tx == nil
	if freshTx {
		tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	to, err := s.ExecutionAt(tx)
	if err != nil {
		return err
	}

	erigonDb := erigon_db.NewErigonDb(tx)
	eridb := db2.NewEriDb(tx)
	smt := smt.NewSMT(eridb)

	// if we are at block 1 then just regenerate the whole thing otherwise take an incremental approach
	var newRoot libcommon.Hash
	if to == 1 {
		newRoot, err = regenerateSequencerIntermediateHashes(s.LogPrefix(), tx, eridb, smt)
	} else {
		// incremental change
	}

	latest, err := rawdb.ReadBlockByNumber(tx, to)
	if err != nil {
		return err
	}
	header := latest.Header()

	receipts, err := rawdb.ReadReceiptsByHash(tx, header.Hash())
	if err != nil {
		return err
	}

	// update the details related to anything that may have changed after figuring out the root
	header.Root = newRoot
	for _, r := range receipts {
		r.PostState = newRoot.Bytes()
	}
	header.ReceiptHash = types.DeriveSha(receipts)
	newHash := header.Hash()

	rawdb.WriteHeader(tx, header)

	err = rawdb.WriteCanonicalHash(tx, newHash, header.Number.Uint64())
	if err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	err = rawdb.WriteReceipts(tx, header.Number.Uint64(), receipts)

	err = erigonDb.WriteBody(header.Number, newHash, latest.Transactions())
	if err != nil {
		return fmt.Errorf("failed to write body: %v", err)
	}

	// write the new block lookup entries
	rawdb.WriteTxLookupEntries(tx, latest)

	if freshTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func UnwindSequencerInterhashsStage(
	u *stagedsync.UnwindState,
	s *stagedsync.StageState,
	tx kv.RwTx,
	ctx context.Context,
	cfg SequencerInterhashesCfg,
	initialCycle bool,
) error {
	return nil
}

func PruneSequencerInterhashesStage(
	s *stagedsync.PruneState,
	tx kv.RwTx,
	cfg SequencerInterhashesCfg,
	ctx context.Context,
	initialCycle bool,
) error {
	return nil
}

func regenerateSequencerIntermediateHashes(logPrefix string, db kv.RwTx, eridb *db2.EriDb, smtIn *smt.SMT) (libcommon.Hash, error) {
	var a *accounts.Account
	var addr libcommon.Address
	var as map[string]string
	var inc uint64

	psr := state2.NewPlainStateReader(db)

	log.Info(fmt.Sprintf("[%s] Collecting account data...", logPrefix))
	dataCollectStartTime := time.Now()
	keys := []utils.NodeKey{}

	// get total accounts count for progress printer
	total := uint64(0)
	if err := psr.ForEach(kv.PlainState, nil, func(k, acc []byte) error {
		total++
		return nil
	}); err != nil {
		return trie.EmptyRoot, err
	}

	progCt := uint64(0)
	err := psr.ForEach(kv.PlainState, nil, func(k, acc []byte) error {
		progCt++
		var err error
		if len(k) == 20 {
			if a != nil { // don't run process on first loop for first account (or it will miss collecting storage)
				keys, err = processAccount(eridb, a, as, inc, psr, addr, keys)
				if err != nil {
					return err
				}
			}

			a = &accounts.Account{}

			if err := a.DecodeForStorage(acc); err != nil {
				// TODO: not an account?
				as = make(map[string]string)
				return nil
			}
			addr = libcommon.BytesToAddress(k)
			inc = a.Incarnation
			// empty storage of previous account
			as = make(map[string]string)
		} else { // otherwise we're reading storage
			_, incarnation, key := dbutils.PlainParseCompositeStorageKey(k)
			if incarnation != inc {
				return nil
			}

			sk := fmt.Sprintf("0x%032x", key)
			v := fmt.Sprintf("0x%032x", acc)

			as[sk] = fmt.Sprint(TrimHexString(v))
		}
		return nil
	})

	if err != nil {
		return trie.EmptyRoot, err
	}

	// process the final account
	keys, err = processAccount(eridb, a, as, inc, psr, addr, keys)
	if err != nil {
		return trie.EmptyRoot, err
	}

	dataCollectTime := time.Since(dataCollectStartTime)
	log.Info(fmt.Sprintf("[%s] Collecting account data finished in %v", logPrefix, dataCollectTime))

	// generate tree
	if _, err := smtIn.GenerateFromKVBulk(logPrefix, keys); err != nil {
		return trie.EmptyRoot, err
	}

	err2 := db.ClearBucket("HermezSmtAccountValues")
	if err2 != nil {
		log.Warn(fmt.Sprint("regenerate SaveStageProgress to zero error: ", err2))
	}

	root := smtIn.LastRoot()
	err = eridb.CommitBatch()
	if err != nil {
		return trie.EmptyRoot, err
	}

	return libcommon.BigToHash(root), nil
}
