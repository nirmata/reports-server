package db

import (
	"database/sql"
	"sync"
)

type MultiDB struct {
	sync.Mutex
	PrimaryDB      *sql.DB
	ReadReplicaDBs []*sql.DB
}

func NewMultiDB(primaryDB *sql.DB, readReplicaDBs []*sql.DB) *MultiDB {
	return &MultiDB{
		PrimaryDB:      primaryDB,
		ReadReplicaDBs: readReplicaDBs,
	}
}
