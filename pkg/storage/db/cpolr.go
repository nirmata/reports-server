package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/kyverno/reports-server/pkg/storage/api"
	"k8s.io/klog/v2"
	"sigs.k8s.io/wg-policy-prototypes/policy-report/pkg/api/wgpolicyk8s.io/v1alpha2"
)

type cpolrdb struct {
	sync.Mutex
	DB        *MultiDB
	clusterId string
}

func NewClusterPolicyReportStore(DB *MultiDB, clusterId string) (api.ClusterPolicyReportsInterface, error) {
	_, err := primaryDB.Exec("CREATE TABLE IF NOT EXISTS clusterpolicyreports (name VARCHAR NOT NULL, clusterId VARCHAR NOT NULL, report JSONB NOT NULL, PRIMARY KEY(name, clusterId))")
	if err != nil {
		klog.ErrorS(err, "failed to create table")
		return nil, err
	}

	_, err = primaryDB.Exec("CREATE INDEX IF NOT EXISTS clusterpolicyreportcluster ON clusterpolicyreports(clusterId)")
	if err != nil {
		klog.ErrorS(err, "failed to create index")
		return nil, err
	}

	return &cpolrdb{primaryDB: primaryDB, readReplicaDBs: readReplicaDBs, clusterId: clusterId}, nil
}

func (c *cpolrdb) List(ctx context.Context) ([]*v1alpha2.ClusterPolicyReport, error) {
	klog.Infof("listing all values")
	res := make([]*v1alpha2.ClusterPolicyReport, 0)
	var jsonb string

	rows, err := c.ReadQuery(ctx, "SELECT report FROM clusterpolicyreports WHERE clusterId = $1", c.clusterId)
	if err != nil {
		klog.ErrorS(err, "failed to list clusterpolicyreports")
		return nil, fmt.Errorf("clusterpolicyreport list: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&jsonb); err != nil {
			klog.ErrorS(err, "failed to scan rows")
			return nil, fmt.Errorf("clusterpolicyreport list: %v", err)
		}
		var report v1alpha2.ClusterPolicyReport
		err := json.Unmarshal([]byte(jsonb), &report)
		if err != nil {
			klog.ErrorS(err, "failed to unmarshal clusterpolicyreport")
			return nil, fmt.Errorf("clusterpolicyreport list: cannot convert jsonb to clusterpolicyreport: %v", err)
		}
		res = append(res, &report)
	}

	klog.Infof("list found length: %d", len(res))
	return res, nil
}

func (c *cpolrdb) Get(ctx context.Context, name string) (*v1alpha2.ClusterPolicyReport, error) {
	var jsonb string

	row := c.ReadQueryRow(ctx, "SELECT report FROM clusterpolicyreports WHERE (name = $1) AND (clusterId = $2)", name, c.clusterId)
	if err := row.Scan(&jsonb); err != nil {
		klog.ErrorS(err, fmt.Sprintf("clusterpolicyreport not found name=%s", name))
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("clusterpolicyreport get %s: no such policy report", name)
		}
		return nil, fmt.Errorf("clusterpolicyreport get %s: %v", name, err)
	}

	var report v1alpha2.ClusterPolicyReport
	err := json.Unmarshal([]byte(jsonb), &report)
	if err != nil {
		klog.ErrorS(err, "failed to unmarshal report")
		return nil, fmt.Errorf("clusterpolicyreport list: cannot convert jsonb to policyreport: %v", err)
	}
	return &report, nil
}

func (c *cpolrdb) Create(ctx context.Context, cpolr *v1alpha2.ClusterPolicyReport) error {
	c.Lock()
	defer c.Unlock()

	if cpolr == nil {
		return errors.New("invalid cluster policy report")
	}

	klog.Infof("creating entry for key:%s", cpolr.Name)
	jsonb, err := json.Marshal(*cpolr)
	if err != nil {
		klog.ErrorS(err, "failed to unmarshal cpolr")
		return err
	}

	_, err = c.primaryDB.Exec("INSERT INTO clusterpolicyreports (name, report, clusterId) VALUES ($1, $2, $3)", cpolr.Name, string(jsonb), c.clusterId)
	if err != nil {
		klog.ErrorS(err, "failed to crate cpolr")
		return fmt.Errorf("create clusterpolicyreport: %v", err)
	}
	return nil
}

func (c *cpolrdb) Update(ctx context.Context, cpolr *v1alpha2.ClusterPolicyReport) error {
	c.Lock()
	defer c.Unlock()

	if cpolr == nil {
		return errors.New("invalid cluster policy report")
	}

	jsonb, err := json.Marshal(*cpolr)
	if err != nil {
		return err
	}

	_, err = c.primaryDB.Exec("UPDATE clusterpolicyreports SET report = $1 WHERE (name = $2) AND (clusterId = $3)", string(jsonb), cpolr.Name, c.clusterId)
	if err != nil {
		klog.ErrorS(err, "failed to updates cpolr")
		return fmt.Errorf("update clusterpolicyreport: %v", err)
	}
	return nil
}

func (c *cpolrdb) Delete(ctx context.Context, name string) error {
	c.Lock()
	defer c.Unlock()

	_, err := c.primaryDB.Exec("DELETE FROM clusterpolicyreports WHERE (name = $1) AND (clusterId = $2)", name, c.clusterId)
	if err != nil {
		klog.ErrorS(err, "failed to delete cpolr")
		return fmt.Errorf("delete clusterpolicyreport: %v", err)
	}
	return nil
}

func (c *cpolrdb) ReadQuery(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	c.Lock()
	replicas := append([]*sql.DB(nil), c.readReplicaDBs...)
	c.Unlock()

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
	return c.primaryDB.Query(query, args...)
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
