package db

import (
	"context"
	"database/sql"
	"math/rand"
	"sync"
	"time"

	"k8s.io/klog/v2"
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

func (m *MultiDB) ReadQuery(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	m.Lock()
	replicas := append([]*sql.DB(nil), m.ReadReplicaDBs...)
	m.Unlock()

	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	rng.Shuffle(len(replicas), func(i, j int) { replicas[i], replicas[j] = replicas[j], replicas[i] })

	for _, readReplicaDB := range replicas {
		rows, err := readReplicaDB.Query(query, args...)
		if err != nil {
			klog.ErrorS(err, "failed to query read replica due to : ", err)
			klog.Info("retrying with next read replica")
			continue
		}
		return rows, nil
	}

	klog.Info("no read replicas available, querying primary db")
	return m.PrimaryDB.Query(query, args...)
}

func (c *cpolrdb) ReadQueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	c.Lock()
	replicas := append([]*sql.DB(nil), c.readReplicaDBs...)
	c.Unlock()

	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	rng.Shuffle(len(replicas), func(i, j int) { replicas[i], replicas[j] = replicas[j], replicas[i] })

	for _, readReplicaDB := range replicas {
		row := readReplicaDB.QueryRow(query, args...)
		if row.Err() != nil {
			klog.ErrorS(row.Err(), "failed to query read replica due to : ", row.Err())
			klog.Info("retrying with next read replica")
			continue
		}

		return row
	}

	klog.Info("no read replicas available, querying primary db")
	return c.primaryDB.QueryRow(query, args...)
}
