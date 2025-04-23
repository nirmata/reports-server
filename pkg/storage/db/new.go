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
	klog.Infof("starting postgres db (primary host %q)", config.Host)

	primaryDB, err := sql.Open("pgx", config.String())
	if err != nil {
		klog.ErrorS(err, "failed to open primary db")
		return nil, err
	}
	if err := pingWithRetry(primaryDB); err != nil {
		return nil, err
	}
	klog.Info("successfully connected to primary db")

	var readReplicas []*sql.DB
	for _, host := range config.ReadReplicaHosts {
		replicaCfg := *config
		replicaCfg.Host = host
		dsn := replicaCfg.String()

		klog.Infof("starting postgres read‑replica db (host %q)", host)
		replicaDB, err := sql.Open("pgx", dsn)
		if err != nil {
			klog.ErrorS(err, "failed to open replica db", "host", host)
			return nil, err
		}
		if err := pingWithRetry(replicaDB); err != nil {
			return nil, err
		}
		klog.Infof("connected to replica %q", host)
		readReplicas = append(readReplicas, replicaDB)
	}

	multiDB := NewMultiDB(primaryDB, readReplicas)

	klog.Info("starting reports store")
	polrstore, err := NewPolicyReportStore(multiDB, clusterId)
	if err != nil {
		return nil, fmt.Errorf("policy report store: %w", err)
	}

	klog.Info("starting cluster policy report store")
	cpolrstore, err := NewClusterPolicyReportStore(multiDB, clusterId)
	if err != nil {
		return nil, fmt.Errorf("cluster policy report store: %w", err)
	}

	klog.Info("starting ephemeral report store")
	ephrstore, err := NewEphemeralReportStore(multiDB, clusterId)
	if err != nil {
		return nil, fmt.Errorf("ephemeral report store: %w", err)
	}

	klog.Info("starting cluster ephemeral report store")
	cephrstore, err := NewClusterEphemeralReportStore(multiDB, clusterId)
	if err != nil {
		return nil, fmt.Errorf("cluster ephemeral report store: %w", err)
	}

	klog.Info("successfully setup storage")
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
		klog.Infof("pinging db (attempt %d/%d)", i, maxRetries)
		if err := db.PingContext(context.Background()); err != nil {
			klog.ErrorS(err, "ping failed")
			time.Sleep(sleepDuration)
			continue
		}
		return nil
	}
	return fmt.Errorf("could not connect after %d attempts", maxRetries)
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
	if err := p.db.PingContext(context.Background()); err != nil {
		klog.ErrorS(err, "failed to ping primary db")
		return false
	}
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
