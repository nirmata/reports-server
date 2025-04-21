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

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/reports-server/pkg/storage/api"
	"k8s.io/klog/v2"
)

type ephrdb struct {
	sync.Mutex
	primaryDB      *sql.DB
	readReplicaDBs []*sql.DB
	clusterId      string
}

func NewEphemeralReportStore(primaryDB *sql.DB, readReplicaDBs []*sql.DB, clusterId string) (api.EphemeralReportsInterface, error) {
	_, err := primaryDB.Exec("CREATE TABLE IF NOT EXISTS ephemeralreports (name VARCHAR NOT NULL, namespace VARCHAR NOT NULL, clusterId VARCHAR NOT NULL, report JSONB NOT NULL, PRIMARY KEY(name, namespace, clusterId))")
	if err != nil {
		klog.ErrorS(err, "failed to create table")
		return nil, err
	}

	_, err = primaryDB.Exec("CREATE INDEX IF NOT EXISTS ephemeralreportnamespace ON ephemeralreports(namespace)")
	if err != nil {
		klog.ErrorS(err, "failed to create index")
		return nil, err
	}

	_, err = primaryDB.Exec("CREATE INDEX IF NOT EXISTS ephemeralreportcluster ON ephemeralreports(clusterId)")
	if err != nil {
		klog.ErrorS(err, "failed to create index")
		return nil, err
	}
	return &ephrdb{primaryDB: primaryDB, readReplicaDBs: readReplicaDBs, clusterId: clusterId}, nil
}

func (p *ephrdb) List(ctx context.Context, namespace string) ([]*reportsv1.EphemeralReport, error) {
	klog.Infof("listing all values for namespace:%s", namespace)
	res := make([]*reportsv1.EphemeralReport, 0)
	var jsonb string
	var rows *sql.Rows
	var err error

	if len(namespace) == 0 {
		rows, err = p.ReadQuery(ctx, "SELECT report FROM ephemeralreports WHERE clusterId = $1", p.clusterId)
	} else {
		rows, err = p.ReadQuery(ctx, "SELECT report FROM ephemeralreports WHERE namespace = $1 AND clusterId = $2", namespace, p.clusterId)
	}
	if err != nil {
		klog.ErrorS(err, "ephemeralreport list: ")
		return nil, fmt.Errorf("ephemeralreport list %q: %v", namespace, err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&jsonb); err != nil {
			klog.ErrorS(err, "ephemeralreport scan failed")
			return nil, fmt.Errorf("ephemeralreport list %q: %v", namespace, err)
		}
		var report reportsv1.EphemeralReport
		err := json.Unmarshal([]byte(jsonb), &report)
		if err != nil {
			klog.ErrorS(err, "cannot convert jsonb to ephemeralreport")
			return nil, fmt.Errorf("ephemeralreport list %q: cannot convert jsonb to ephemeralreport: %v", namespace, err)
		}
		res = append(res, &report)
	}

	klog.Infof("list found length: %d", len(res))
	return res, nil
}

func (p *ephrdb) Get(ctx context.Context, name, namespace string) (*reportsv1.EphemeralReport, error) {
	var jsonb string

	row := p.ReadQueryRow(ctx, "SELECT report FROM ephemeralreports WHERE (namespace = $1) AND (name = $2) AND (clusterId = $3)", namespace, name, p.clusterId)
	if err := row.Scan(&jsonb); err != nil {
		klog.ErrorS(err, fmt.Sprintf("ephemeralreport not found name=%s namespace=%s", name, namespace))
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ephemeralreport get %s/%s: no such ephemeral report: %v", namespace, name, err)
		}
		return nil, fmt.Errorf("ephemeralreport get %s/%s: %v", namespace, name, err)
	}

	var report reportsv1.EphemeralReport
	err := json.Unmarshal([]byte(jsonb), &report)
	if err != nil {
		klog.ErrorS(err, "cannot convert jsonb to ephemeralreport")
		return nil, fmt.Errorf("ephemeralreport list %q: cannot convert jsonb to ephemeralreport: %v", namespace, err)
	}
	return &report, nil
}

func (p *ephrdb) Create(ctx context.Context, polr *reportsv1.EphemeralReport) error {
	p.Lock()
	defer p.Unlock()

	if polr == nil {
		return errors.New("invalid ephemeral report")
	}

	klog.Infof("creating entry for key:%s/%s", polr.Name, polr.Namespace)
	jsonb, err := json.Marshal(*polr)
	if err != nil {
		return err
	}

	_, err = p.primaryDB.Exec("INSERT INTO ephemeralreports (name, namespace, report, clusterId) VALUES ($1, $2, $3, $4)", polr.Name, polr.Namespace, string(jsonb), p.clusterId)
	if err != nil {
		klog.ErrorS(err, "failed to create ephemeral report")
		return fmt.Errorf("create ephemeralreport: %v", err)
	}
	return nil
}

func (p *ephrdb) Update(ctx context.Context, polr *reportsv1.EphemeralReport) error {
	p.Lock()
	defer p.Unlock()

	if polr == nil {
		return errors.New("invalid ephemeral report")
	}

	jsonb, err := json.Marshal(*polr)
	if err != nil {
		return err
	}

	_, err = p.primaryDB.Exec("UPDATE ephemeralreports SET report = $1 WHERE (namespace = $2) AND (name = $3) AND (clusterId = $4)", string(jsonb), polr.Namespace, polr.Name, p.clusterId)
	if err != nil {
		klog.ErrorS(err, "failed to update ephemeral report")
		return fmt.Errorf("update ephemeralreport: %v", err)
	}
	return nil
}

func (p *ephrdb) Delete(ctx context.Context, name, namespace string) error {
	p.Lock()
	defer p.Unlock()

	_, err := p.primaryDB.Exec("DELETE FROM ephemeralreports WHERE (namespace = $1) AND (name = $2) AND (clusterId = $3)", namespace, name, p.clusterId)
	if err != nil {
		klog.ErrorS(err, "failed to delete ephemeral report")
		return fmt.Errorf("delete ephemeralreport: %v", err)
	}
	return nil
}

func (c *ephrdb) ReadQuery(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
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

func (c *ephrdb) ReadQueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
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
