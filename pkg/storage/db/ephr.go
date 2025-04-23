package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/reports-server/pkg/storage/api"
	"k8s.io/klog/v2"
)

type ephrdb struct {
	sync.Mutex
	MultiDB   *MultiDB
	clusterId string
}

func NewEphemeralReportStore(MultiDB *MultiDB, clusterId string) (api.EphemeralReportsInterface, error) {
	klog.Infof("DB: Initializing EphemeralReportStore for cluster: %s", clusterId)
	_, err := MultiDB.PrimaryDB.Exec("CREATE TABLE IF NOT EXISTS ephemeralreports (name VARCHAR NOT NULL, namespace VARCHAR NOT NULL, clusterId VARCHAR NOT NULL, report JSONB NOT NULL, PRIMARY KEY(name, namespace, clusterId))")
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create ephemeralreports table")
		return nil, err
	}

	_, err = MultiDB.PrimaryDB.Exec("CREATE INDEX IF NOT EXISTS ephemeralreportnamespace ON ephemeralreports(namespace)")
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create ephemeralreports namespace index")
		return nil, err
	}

	_, err = MultiDB.PrimaryDB.Exec("CREATE INDEX IF NOT EXISTS ephemeralreportcluster ON ephemeralreports(clusterId)")
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create ephemeralreports cluster index")
		return nil, err
	}
	klog.Infof("DB: Successfully initialized EphemeralReportStore for cluster: %s", clusterId)
	return &ephrdb{MultiDB: MultiDB, clusterId: clusterId}, nil
}

func (p *ephrdb) List(ctx context.Context, namespace string) ([]*reportsv1.EphemeralReport, error) {
	klog.Infof("DB: Starting List operation for EphemeralReports with namespace:%s clusterId:%s", namespace, p.clusterId)
	res := make([]*reportsv1.EphemeralReport, 0)
	var jsonb string
	var rows *sql.Rows
	var err error

	if len(namespace) == 0 {
		klog.Infof("DB: Executing read query for all namespaces in cluster: %s", p.clusterId)
		rows, err = p.MultiDB.ReadQuery(ctx, "SELECT report FROM ephemeralreports WHERE clusterId = $1", p.clusterId)
	} else {
		klog.Infof("DB: Executing read query for namespace:%s in cluster: %s", namespace, p.clusterId)
		rows, err = p.MultiDB.ReadQuery(ctx, "SELECT report FROM ephemeralreports WHERE namespace = $1 AND clusterId = $2", namespace, p.clusterId)
	}
	if err != nil {
		klog.ErrorS(err, "DB: Failed to list ephemeralreports")
		return nil, fmt.Errorf("ephemeralreport list %q: %v", namespace, err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		count++
		if err := rows.Scan(&jsonb); err != nil {
			klog.ErrorS(err, "DB: Failed to scan row %d", count)
			return nil, fmt.Errorf("ephemeralreport list %q: %v", namespace, err)
		}
		var report reportsv1.EphemeralReport
		err := json.Unmarshal([]byte(jsonb), &report)
		if err != nil {
			klog.ErrorS(err, "DB: Failed to unmarshal ephemeralreport for row %d", count)
			return nil, fmt.Errorf("ephemeralreport list %q: cannot convert jsonb to ephemeralreport: %v", namespace, err)
		}
		res = append(res, &report)
	}

	klog.Infof("DB: List operation completed. Successfully retrieved %d reports", len(res))
	return res, nil
}

func (p *ephrdb) Get(ctx context.Context, name, namespace string) (*reportsv1.EphemeralReport, error) {
	klog.Infof("DB: Starting Get operation for EphemeralReport name=%s namespace=%s clusterId=%s", name, namespace, p.clusterId)
	var jsonb string

	row := p.MultiDB.ReadQueryRow(ctx, "SELECT report FROM ephemeralreports WHERE (namespace = $1) AND (name = $2) AND (clusterId = $3)", namespace, name, p.clusterId)
	if err := row.Scan(&jsonb); err != nil {
		klog.ErrorS(err, "DB: EphemeralReport not found name=%s namespace=%s clusterId=%s", name, namespace, p.clusterId)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("ephemeralreport get %s/%s: no such ephemeral report: %v", namespace, name, err)
		}
		return nil, fmt.Errorf("ephemeralreport get %s/%s: %v", namespace, name, err)
	}

	var report reportsv1.EphemeralReport
	err := json.Unmarshal([]byte(jsonb), &report)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to unmarshal report")
		return nil, fmt.Errorf("ephemeralreport list %q: cannot convert jsonb to ephemeralreport: %v", namespace, err)
	}
	klog.Infof("DB: Successfully retrieved EphemeralReport name=%s namespace=%s", name, namespace)
	return &report, nil
}

func (p *ephrdb) Create(ctx context.Context, polr *reportsv1.EphemeralReport) error {
	p.Lock()
	defer p.Unlock()

	if polr == nil {
		return errors.New("invalid ephemeral report")
	}

	klog.Infof("DB: Creating entry in primary database for key:%s/%s", polr.Name, polr.Namespace)
	jsonb, err := json.Marshal(*polr)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to marshal ephemeral report")
		return err
	}

	_, err = p.MultiDB.PrimaryDB.Exec("INSERT INTO ephemeralreports (name, namespace, report, clusterId) VALUES ($1, $2, $3, $4)", polr.Name, polr.Namespace, string(jsonb), p.clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create ephemeral report in primary database")
		return fmt.Errorf("create ephemeralreport: %v", err)
	}
	klog.Infof("DB: Successfully created entry in primary database for key:%s/%s", polr.Name, polr.Namespace)
	return nil
}

func (p *ephrdb) Update(ctx context.Context, polr *reportsv1.EphemeralReport) error {
	p.Lock()
	defer p.Unlock()

	if polr == nil {
		return errors.New("invalid ephemeral report")
	}

	klog.Infof("DB: Updating entry in primary database for key:%s/%s", polr.Name, polr.Namespace)
	jsonb, err := json.Marshal(*polr)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to marshal ephemeral report")
		return err
	}

	_, err = p.MultiDB.PrimaryDB.Exec("UPDATE ephemeralreports SET report = $1 WHERE (namespace = $2) AND (name = $3) AND (clusterId = $4)", string(jsonb), polr.Namespace, polr.Name, p.clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to update ephemeral report in primary database")
		return fmt.Errorf("update ephemeralreport: %v", err)
	}
	klog.Infof("DB: Successfully updated entry in primary database for key:%s/%s", polr.Name, polr.Namespace)
	return nil
}

func (p *ephrdb) Delete(ctx context.Context, name, namespace string) error {
	p.Lock()
	defer p.Unlock()

	klog.Infof("DB: Deleting entry from primary database for key:%s/%s", name, namespace)
	_, err := p.MultiDB.PrimaryDB.Exec("DELETE FROM ephemeralreports WHERE (namespace = $1) AND (name = $2) AND (clusterId = $3)", namespace, name, p.clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to delete ephemeral report from primary database")
		return fmt.Errorf("delete ephemeralreport: %v", err)
	}
	klog.Infof("DB: Successfully deleted entry from primary database for key:%s/%s", name, namespace)
	return nil
}
