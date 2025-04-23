package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx driver
	"github.com/kyverno/reports-server/pkg/storage/api"
	"k8s.io/klog/v2"
)

const (
	maxRetries    = 10
	sleepDuration = 15 * time.Second
)

func New(config *PostgresConfig, clusterId string) (api.Storage, error) {
	klog.Infof("DB: Starting database initialization for cluster: %s", clusterId)
	klog.Infof("DB: Connecting to primary database at host: %s", config.Host)

	primaryDB, err := sql.Open("pgx", config.String())
	if err != nil {
		klog.ErrorS(err, "DB: Failed to open primary database connection")
		return nil, err
	}
	if err := pingWithRetry(primaryDB); err != nil {
		klog.ErrorS(err, "DB: Failed to establish connection with primary database")
		return nil, err
	}
	klog.Info("DB: Successfully connected to primary database")

	var readReplicas []*sql.DB
	for i, host := range config.ReadReplicaHosts {
		replicaCfg := *config
		replicaCfg.Host = host
		dsn := replicaCfg.String()

		klog.Infof("DB: Connecting to read replica %d at host: %s", i+1, host)
		replicaDB, err := sql.Open("pgx", dsn)
		if err != nil {
			klog.ErrorS(err, "DB: Failed to open read replica connection", "host", host)
			return nil, err
		}
		if err := pingWithRetry(replicaDB); err != nil {
			klog.ErrorS(err, "DB: Failed to establish connection with read replica", "host", host)
			return nil, err
		}
		klog.Infof("DB: Successfully connected to read replica %d at host: %s", i+1, host)
		readReplicas = append(readReplicas, replicaDB)
	}

	multiDB := NewMultiDB(primaryDB, readReplicas)

	klog.Info("DB: Initializing PolicyReportStore")
	polrstore, err := NewPolicyReportStore(multiDB, clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to initialize PolicyReportStore")
		return nil, fmt.Errorf("policy report store: %w", err)
	}

	klog.Info("DB: Initializing ClusterPolicyReportStore")
	cpolrstore, err := NewClusterPolicyReportStore(multiDB, clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to initialize ClusterPolicyReportStore")
		return nil, fmt.Errorf("cluster policy report store: %w", err)
	}

	klog.Info("DB: Initializing EphemeralReportStore")
	ephrstore, err := NewEphemeralReportStore(multiDB, clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to initialize EphemeralReportStore")
		return nil, fmt.Errorf("ephemeral report store: %w", err)
	}

	klog.Info("DB: Initializing ClusterEphemeralReportStore")
	cephrstore, err := NewClusterEphemeralReportStore(multiDB, clusterId)
	if err != nil {
		klog.ErrorS(err, "DB: Failed to initialize ClusterEphemeralReportStore")
		return nil, fmt.Errorf("cluster ephemeral report store: %w", err)
	}

	klog.Info("DB: Successfully completed database initialization")
	return &postgresstore{
		db:         primaryDB,
		polrstore:  polrstore,
		cpolrstore: cpolrstore,
		ephrstore:  ephrstore,
		cephrstore: cephrstore,
	}, nil
}

// pingWithRetry tries db.PingContext up to maxRetries with sleep.
func pingWithRetry(db *sql.DB) error {
	for i := 1; i <= maxRetries; i++ {
		klog.Infof("DB: Pinging database (attempt %d/%d)", i, maxRetries)
		if err := db.PingContext(context.Background()); err != nil {
			klog.ErrorS(err, "DB: Ping failed")
			time.Sleep(sleepDuration)
			continue
		}
		klog.Info("DB: Ping successful")
		return nil
	}
	return fmt.Errorf("DB: Could not connect after %d attempts", maxRetries)
}

type postgresstore struct {
	db         *sql.DB
	polrstore  api.PolicyReportsInterface
	cpolrstore api.ClusterPolicyReportsInterface
	ephrstore  api.EphemeralReportsInterface
	cephrstore api.ClusterEphemeralReportsInterface
}

func (p *postgresstore) ClusterPolicyReports() api.ClusterPolicyReportsInterface {
	return p.cpolrstore
}

func (p *postgresstore) PolicyReports() api.PolicyReportsInterface {
	return p.polrstore
}

func (p *postgresstore) ClusterEphemeralReports() api.ClusterEphemeralReportsInterface {
	return p.cephrstore
}

func (p *postgresstore) EphemeralReports() api.EphemeralReportsInterface {
	return p.ephrstore
}

func (p *postgresstore) Ready() bool {
	klog.Info("DB: Checking database readiness")
	if err := p.db.PingContext(context.Background()); err != nil {
		klog.ErrorS(err, "DB: Failed to ping primary database")
		return false
	}
	klog.Info("DB: Database is ready")
	return true
}

type PostgresConfig struct {
	Host             string
	ReadReplicaHosts []string
	Port             int
	User             string
	Password         string
	DBname           string
	SSLMode          string
	SSLRootCert      string
	SSLKey           string
	SSLCert          string
}

func (p PostgresConfig) String() string {
	hosts := strings.Split(p.Host, ",")
	if p.Port != 0 {
		for i, h := range hosts {
			hosts[i] = fmt.Sprintf("%s:%d", h, p.Port)
		}
	}
	hostPart := strings.Join(hosts, ",")

	// build the base DSN
	dsn := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=%s",
		p.User, p.Password, hostPart, p.DBname, p.SSLMode,
	)

	if p.SSLRootCert != "" {
		dsn += "&sslrootcert=" + p.SSLRootCert
	}
	if p.SSLKey != "" {
		dsn += "&sslkey=" + p.SSLKey
	}
	if p.SSLCert != "" {
		dsn += "&sslcert=" + p.SSLCert
	}
	return dsn
}
