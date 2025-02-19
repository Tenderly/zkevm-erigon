package stagedsync

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/tenderly/zkevm-erigon/eth/stagedsync/stages"

	libcommon "github.com/tenderly/zkevm-erigon-lib/common"
	"github.com/tenderly/zkevm-erigon-lib/etl"
	"github.com/tenderly/zkevm-erigon-lib/kv"
	"github.com/tenderly/zkevm-erigon/chain"

	"github.com/tenderly/zkevm-erigon/common/dbutils"
)

func extractHeaders(k []byte, v []byte, next etl.ExtractNextFunc) error {
	// We only want to extract entries composed by Block Number + Header Hash
	if len(k) != 40 {
		return nil
	}
	return next(k, libcommon.Copy(k[8:]), libcommon.Copy(k[:8]))
}

type BlockHashesCfg struct {
	db     kv.RwDB
	tmpDir string
	cc     *chain.Config
}

func StageBlockHashesCfg(db kv.RwDB, tmpDir string, cc *chain.Config) BlockHashesCfg {
	return BlockHashesCfg{
		db:     db,
		tmpDir: tmpDir,
		cc:     cc,
	}
}

func SpawnBlockHashStage(s *StageState, tx kv.RwTx, cfg BlockHashesCfg, ctx context.Context) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}
	quit := ctx.Done()
	headNumber, err := stages.GetStageProgress(tx, stages.Headers)
	if err != nil {
		return fmt.Errorf("getting headers progress: %w", err)
	}
	if s.BlockNumber == headNumber {
		return nil
	}

	startKey := make([]byte, 8)
	binary.BigEndian.PutUint64(startKey, s.BlockNumber)
	endKey := dbutils.HeaderKey(headNumber+1, libcommon.Hash{}) // etl.Tranform uses ExractEndKey as exclusive bound, therefore +1

	//todo do we need non canonical headers ?
	logPrefix := s.LogPrefix()
	if err := etl.Transform(
		logPrefix,
		tx,
		kv.Headers,
		kv.HeaderNumber,
		cfg.tmpDir,
		extractHeaders,
		etl.IdentityLoadFunc,
		etl.TransformArgs{
			ExtractStartKey: startKey,
			ExtractEndKey:   endKey,
			Quit:            quit,
		},
	); err != nil {
		return err
	}
	if err = s.Update(tx, headNumber); err != nil {
		return err
	}
	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func UnwindBlockHashStage(u *UnwindState, tx kv.RwTx, cfg BlockHashesCfg, ctx context.Context) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if err = u.Done(tx); err != nil {
		return fmt.Errorf(" reset: %w", err)
	}
	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return fmt.Errorf("failed to write db commit: %w", err)
		}
	}
	return nil
}

func PruneBlockHashStage(p *PruneState, tx kv.RwTx, cfg BlockHashesCfg, ctx context.Context) (err error) {
	useExternalTx := tx != nil
	if !useExternalTx {
		tx, err = cfg.db.BeginRw(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback()
	}

	if !useExternalTx {
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
