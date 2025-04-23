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

type cephr struct {
	sync.Mutex
	clusterId string
	MultiDB   *MultiDB
}

func NewClusterEphemeralReportStore(MultiDB *MultiDB, clusterId string) (api.ClusterEphemeralReportsInterface, error) {
	klog.Infof("DB: Initializing ClusterEphemeralReportStore for cluster: %s", clusterId)
	_, err := MultiDB.PrimaryDB.Exec("CREATE TABLE IF NOT EXISTS clusterephemeralreports (name VARCHAR NOT NULL, clusterId VARCHAR NOT NULL, report JSONB NOT NULL, PRIMARY KEY(name, clusterId))")
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create clusterephemeralreports table")
		return nil, err
	}

	_, err = MultiDB.PrimaryDB.Exec("CREATE INDEX IF NOT EXISTS clusterephemeralreportcluster ON clusterephemeralreports(clusterId)")
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create clusterephemeralreports index")
		return nil, err
	}

	klog.Infof("DB: Successfully initialized ClusterEphemeralReportStore for cluster: %s", clusterId)
	return &cephr{MultiDB: MultiDB, clusterId: clusterId}, nil
}

func (c *cephr) List(ctx context.Context) ([]*reportsv1.ClusterEphemeralReport, error) {
	klog.Infof("DB: Starting List operation for ClusterEphemeralReports with clusterId: %s", c.clusterId)
	res := make([]*reportsv1.ClusterEphemeralReport, 0)
	var jsonb string

	klog.Infof("DB: Executing read query for clusterId: %s", c.clusterId)
	rows, err := c.MultiDB.ReadQuery(ctx, "SELECT report FROM clusterephemeralreports WHERE (clusterId = $1)", c.clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to list clusterephemeralreports")
		return nil, fmt.Errorf("clusterephemeralreports list: %v", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		count++
		if err := rows.Scan(&jsonb); err != nil {
			klog.ErrorS(err, "DB: Failed to scan row %d", count)
			return nil, fmt.Errorf("clusterephemeralreports list: %v", err)
		}
		var report reportsv1.ClusterEphemeralReport
		err := json.Unmarshal([]byte(jsonb), &report)
		if err != nil {
			klog.ErrorS(err, "DB: Failed to unmarshal clusterephemeralreports for row %d", count)
			return nil, fmt.Errorf("clusterephemeralreports list: cannot convert jsonb to clusterephemeralreports: %v", err)
		}
		res = append(res, &report)
	}

	klog.Infof("DB: List operation completed. Successfully retrieved %d reports", len(res))
	return res, nil
}

func (c *cephr) Get(ctx context.Context, name string) (*reportsv1.ClusterEphemeralReport, error) {
	klog.Infof("DB: Starting Get operation for ClusterEphemeralReport name=%s clusterId=%s", name, c.clusterId)
	var jsonb string

	row := c.MultiDB.ReadQueryRow(ctx, "SELECT report FROM clusterephemeralreports WHERE (name = $1) AND (clusterId = $2)", name, c.clusterId)
	if err := row.Scan(&jsonb); err != nil {
		klog.ErrorS(err, "DB: ClusterEphemeralReport not found name=%s clusterId=%s", name, c.clusterId)
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("clusterephemeralreport get %s: no such ephemeral report", name)
		}
		return nil, fmt.Errorf("clusterephemeralreport get %s: %v", name, err)
	}

	var report reportsv1.ClusterEphemeralReport
	err := json.Unmarshal([]byte(jsonb), &report)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to unmarshal report")
		return nil, fmt.Errorf("clusterephemeralreport list: cannot convert jsonb to ephemeralreport: %v", err)
	}
	klog.Infof("DB: Successfully retrieved ClusterEphemeralReport name=%s", name)
	return &report, nil
}

func (c *cephr) Create(ctx context.Context, cephr *reportsv1.ClusterEphemeralReport) error {
	c.Lock()
	defer c.Unlock()

	if cephr == nil {
		return errors.New("invalid cluster ephemeral report")
	}

	klog.Infof("DB: Creating entry in primary database for key:%s", cephr.Name)
	jsonb, err := json.Marshal(*cephr)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to marshal cephr")
		return err
	}

	_, err = c.MultiDB.PrimaryDB.Exec("INSERT INTO clusterephemeralreports (name, report, clusterId) VALUES ($1, $2, $3)", cephr.Name, string(jsonb), c.clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to create cephr in primary database")
		return fmt.Errorf("create clusterephemeralreport: %v", err)
	}
	klog.Infof("DB: Successfully created entry in primary database for key:%s", cephr.Name)
	return nil
}

func (c *cephr) Update(ctx context.Context, cephr *reportsv1.ClusterEphemeralReport) error {
	c.Lock()
	defer c.Unlock()

	if cephr == nil {
		return errors.New("invalid cluster ephemeral report")
	}

	klog.Infof("DB: Updating entry in primary database for key:%s", cephr.Name)
	jsonb, err := json.Marshal(*cephr)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to marshal cephr")
		return err
	}

	_, err = c.MultiDB.PrimaryDB.Exec("UPDATE clusterephemeralreports SET report = $1 WHERE (name = $2) AND (clusterId = $3)", string(jsonb), cephr.Name, c.clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to update cephr in primary database")
		return fmt.Errorf("update clusterephemeralreport: %v", err)
	}
	klog.Infof("DB: Successfully updated entry in primary database for key:%s", cephr.Name)
	return nil
}

func (c *cephr) Delete(ctx context.Context, name string) error {
	c.Lock()
	defer c.Unlock()

	klog.Infof("DB: Deleting entry from primary database for key:%s", name)
	_, err := c.MultiDB.PrimaryDB.Exec("DELETE FROM clusterephemeralreports WHERE (name = $1) AND (clusterId = $2)", name, c.clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to delete cephr from primary database")
		return fmt.Errorf("delete clusterephemeralreport: %v", err)
	}
	klog.Infof("DB: Successfully deleted entry from primary database for key:%s", name)
	return nil
}
