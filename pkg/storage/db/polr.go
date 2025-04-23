package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/kyverno/reports-server/pkg/storage/api"
	"k8s.io/klog/v2"
	"sigs.k8s.io/wg-policy-prototypes/policy-report/pkg/api/wgpolicyk8s.io/v1alpha2"
)

type polrdb struct {
	sync.Mutex
	MultiDB   *MultiDB
	clusterId string
}

func NewPolicyReportStore(MultiDB *MultiDB, clusterId string) (api.PolicyReportsInterface, error) {
	klog.Infof("DB: Initializing PolicyReportStore for cluster: %s", clusterId)
	_, err := MultiDB.PrimaryDB.Exec("CREATE TABLE IF NOT EXISTS policyreports (name VARCHAR NOT NULL, namespace VARCHAR NOT NULL, clusterId VARCHAR NOT NULL, report JSONB NOT NULL, PRIMARY KEY(name, namespace, clusterId))")
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create policyreports table")
		return nil, err
	}

	_, err = MultiDB.PrimaryDB.Exec("CREATE INDEX IF NOT EXISTS policyreportnamespace ON policyreports(namespace)")
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create policyreports namespace index")
		return nil, err
	}
	_, err = MultiDB.PrimaryDB.Exec("CREATE INDEX IF NOT EXISTS policyreportcluster ON policyreports(clusterId)")
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create policyreports cluster index")
		return nil, err
	}
	klog.Infof("DB: Successfully initialized PolicyReportStore for cluster: %s", clusterId)
	return &polrdb{MultiDB: MultiDB, clusterId: clusterId}, nil
}

func (p *polrdb) List(ctx context.Context, namespace string) ([]*v1alpha2.PolicyReport, error) {
	klog.Infof("DB: Starting List operation for PolicyReports with namespace:%s clusterId:%s", namespace, p.clusterId)
	res := make([]*v1alpha2.PolicyReport, 0)
	var jsonb string
	var rows *sql.Rows
	var err error

	if len(namespace) == 0 {
		klog.Infof("DB: Executing read query for all namespaces in cluster: %s", p.clusterId)
		rows, err = p.MultiDB.ReadQuery(ctx, "SELECT report FROM policyreports WHERE clusterId = $1", p.clusterId)
	} else {
		klog.Infof("DB: Executing read query for namespace:%s in cluster: %s", namespace, p.clusterId)
		rows, err = p.MultiDB.ReadQuery(ctx, "SELECT report FROM policyreports WHERE namespace = $1 AND clusterId = $2", namespace, p.clusterId)
	}

	if err != nil {
		klog.ErrorS(err, "DB: Failed to list policyreports")
		return nil, fmt.Errorf("policyreport list %q: %v", namespace, err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		count++
		if err := rows.Scan(&jsonb); err != nil {
			klog.ErrorS(err, "DB: Failed to scan row %d", count)
			return nil, fmt.Errorf("policyreport list %q: %v", namespace, err)
		}
		var report v1alpha2.PolicyReport
		err := json.Unmarshal([]byte(jsonb), &report)
		if err != nil {
			klog.ErrorS(err, "DB: Failed to unmarshal policyreport for row %d", count)
			return nil, fmt.Errorf("policyreport list %q: cannot convert jsonb to policyreport: %v", namespace, err)
		}
		res = append(res, &report)
	}

	klog.Infof("DB: List operation completed. Successfully retrieved %d reports", len(res))
	return res, nil
}

func (p *polrdb) Get(ctx context.Context, name, namespace string) (*v1alpha2.PolicyReport, error) {
	klog.Infof("DB: Starting Get operation for PolicyReport name=%s namespace=%s clusterId=%s", name, namespace, p.clusterId)
	var jsonb string

	row := p.MultiDB.ReadQueryRow(ctx, "SELECT report FROM policyreports WHERE (namespace = $1) AND (name = $2) AND (clusterId = $3)", namespace, name, p.clusterId)
	if err := row.Scan(&jsonb); err != nil {
		klog.ErrorS(err, "DB: PolicyReport not found name=%s namespace=%s clusterId=%s", name, namespace, p.clusterId)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("policyreport get %s/%s: no such policy report: %v", namespace, name, err)
		}
		return nil, fmt.Errorf("policyreport get %s/%s: %v", namespace, name, err)
	}

	var report v1alpha2.PolicyReport
	err := json.Unmarshal([]byte(jsonb), &report)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to unmarshal report")
		return nil, fmt.Errorf("policyreport list %q: cannot convert jsonb to policyreport: %v", namespace, err)
	}
	klog.Infof("DB: Successfully retrieved PolicyReport name=%s namespace=%s", name, namespace)
	return &report, nil
}

func (p *polrdb) Create(ctx context.Context, polr *v1alpha2.PolicyReport) error {
	p.Lock()
	defer p.Unlock()

	if polr == nil {
		return errors.New("invalid policy report")
	}

	klog.Infof("DB: Creating entry in primary database for key:%s/%s", polr.Name, polr.Namespace)
	jsonb, err := json.Marshal(*polr)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to marshal policy report")
		return err
	}

	_, err = p.MultiDB.PrimaryDB.Exec("INSERT INTO policyreports (name, namespace, report, clusterId) VALUES ($1, $2, $3, $4)", polr.Name, polr.Namespace, string(jsonb), p.clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create policy report in primary database")
		return fmt.Errorf("create policyreport: %v", err)
	}
	klog.Infof("DB: Successfully created entry in primary database for key:%s/%s", polr.Name, polr.Namespace)
	return nil
}

func (p *polrdb) Update(ctx context.Context, polr *v1alpha2.PolicyReport) error {
	p.Lock()
	defer p.Unlock()

	if polr == nil {
		return errors.New("invalid policy report")
	}

	klog.Infof("DB: Updating entry in primary database for key:%s/%s", polr.Name, polr.Namespace)
	jsonb, err := json.Marshal(*polr)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to marshal policy report")
		return err
	}

	_, err = p.MultiDB.PrimaryDB.Exec("UPDATE policyreports SET report = $1 WHERE (namespace = $2) AND (name = $3) AND (clusterId = $4)", string(jsonb), polr.Namespace, polr.Name, p.clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to update policy report in primary database")
		return fmt.Errorf("update policyreport: %v", err)
	}
	klog.Infof("DB: Successfully updated entry in primary database for key:%s/%s", polr.Name, polr.Namespace)
	return nil
}

func (p *polrdb) Delete(ctx context.Context, name, namespace string) error {
	p.Lock()
	defer p.Unlock()

	klog.Infof("DB: Deleting entry from primary database for key:%s/%s", name, namespace)
	_, err := p.MultiDB.PrimaryDB.Exec("DELETE FROM policyreports WHERE (namespace = $1) AND (name = $2) AND (clusterId = $3)", namespace, name, p.clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to delete policy report from primary database")
		return fmt.Errorf("delete policyreport: %v", err)
	}
	klog.Infof("DB: Successfully deleted entry from primary database for key:%s/%s", name, namespace)
	return nil
}
