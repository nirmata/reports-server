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
	klog.Infof("DB: Starting ReadQuery operation. Query: %s", query)
	m.Lock()
	replicas := append([]*sql.DB(nil), m.ReadReplicaDBs...)
	m.Unlock()

	if len(replicas) == 0 {
		klog.Info("DB: No read replicas available, routing to primary database")
		return m.PrimaryDB.Query(query, args...)
	}

	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	rng.Shuffle(len(replicas), func(i, j int) { replicas[i], replicas[j] = replicas[j], replicas[i] })

	for i, readReplicaDB := range replicas {
		klog.Infof("DB: Attempting read replica %d", i+1)
		rows, err := readReplicaDB.Query(query, args...)
		if err != nil {
			klog.ErrorS(err, "DB: Failed to query read replica %d", i+1)
			continue
		}
		klog.Infof("DB: Successfully routed query to read replica %d", i+1)
		return rows, nil
	}

	klog.Info("DB: All read replicas failed, falling back to primary database")
	return m.PrimaryDB.Query(query, args...)
}

func (m *MultiDB) ReadQueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	klog.Infof("DB: Starting ReadQueryRow operation. Query: %s", query)
	m.Lock()
	replicas := append([]*sql.DB(nil), m.ReadReplicaDBs...)
	m.Unlock()

	if len(replicas) == 0 {
		klog.Info("DB: No read replicas available, routing to primary database")
		return m.PrimaryDB.QueryRow(query, args...)
	}

	source := rand.NewSource(time.Now().UnixNano())
	rng := rand.New(source)
	rng.Shuffle(len(replicas), func(i, j int) { replicas[i], replicas[j] = replicas[j], replicas[i] })

	for i, readReplicaDB := range replicas {
		klog.Infof("DB: Attempting read replica %d", i+1)
		row := readReplicaDB.QueryRow(query, args...)
		if row.Err() != nil {
			klog.ErrorS(row.Err(), "DB: Failed to query read replica %d", i+1)
			continue
		}
		klog.Infof("DB: Successfully routed query to read replica %d", i+1)
		return row
	}

	klog.Info("DB: All read replicas failed, falling back to primary database")
	return m.PrimaryDB.QueryRow(query, args...)
}
