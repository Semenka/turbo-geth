package migrations

import (
	"encoding/binary"
	"fmt"
	"runtime"
	"time"

	"github.com/ledgerwatch/turbo-geth/common"
	"github.com/ledgerwatch/turbo-geth/common/dbutils"
	"github.com/ledgerwatch/turbo-geth/common/etl"
	"github.com/ledgerwatch/turbo-geth/core/types"
	"github.com/ledgerwatch/turbo-geth/ethdb"
	"github.com/ledgerwatch/turbo-geth/ethdb/cbor"
	"github.com/ledgerwatch/turbo-geth/log"
	"github.com/ledgerwatch/turbo-geth/rlp"
	"github.com/valyala/gozstd"
)

var receiptsCborEncode = Migration{
	Name: "receipts_cbor_encode",
	Up: func(db ethdb.Database, datadir string, OnLoadCommit etl.LoadCommitHandler) error {
		logEvery := time.NewTicker(30 * time.Second)
		defer logEvery.Stop()

		buf := make([]byte, 0, 100_000)
		if err := db.Walk(dbutils.BlockReceiptsPrefix, nil, 0, func(k, v []byte) (bool, error) {
			blockNum := binary.BigEndian.Uint64(k[:8])
			select {
			default:
			case <-logEvery.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				log.Info("Migration progress", "blockNum", blockNum, "alloc", common.StorageSize(m.Alloc), "sys", common.StorageSize(m.Sys))
			}

			// Convert the receipts from their storage form to their internal representation
			storageReceipts := []*types.ReceiptForStorage{}
			if err := rlp.DecodeBytes(v, &storageReceipts); err != nil {
				return false, fmt.Errorf("invalid receipt array RLP: %w, k=%x", err, k)
			}

			buf = buf[:0]
			if err := cbor.Marshal(&buf, storageReceipts); err != nil {
				return false, err
			}
			return true, db.Put(dbutils.BlockReceiptsPrefix, common.CopyBytes(k), common.CopyBytes(buf))
		}); err != nil {
			return err
		}

		return OnLoadCommit(db, nil, true)
	},
}

//nolint:deadcode,unused,varcheck
var receiptsZstdCompression = Migration{
	Name: "receipts_zstd_compression",
	Up: func(db ethdb.Database, datadir string, OnLoadCommit etl.LoadCommitHandler) error {
		logEvery := time.NewTicker(30 * time.Second)
		defer logEvery.Stop()

		if err := db.Walk(dbutils.BlockReceiptsPrefix, nil, 0, func(k, v []byte) (bool, error) {
			blockNum := binary.BigEndian.Uint64(k[:8])
			select {
			default:
			case <-logEvery.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				log.Info("Migration progress", "blockNum", blockNum, "alloc", common.StorageSize(m.Alloc), "sys", common.StorageSize(m.Sys))
			}

			newV := gozstd.CompressDict(nil, v, dbutils.CompressionDicts.CReceipts)
			return true, db.Put(dbutils.BlockReceiptsPrefix, common.CopyBytes(k), newV)
		}); err != nil {
			return err
		}

		return OnLoadCommit(db, nil, true)
	},
}
