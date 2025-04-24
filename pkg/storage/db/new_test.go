package db

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		config         *PostgresConfig
		clusterId      string
		setupMocks     func(*sqlmock.Sqlmock)
		expectedError  error
		expectedStores bool
	}{
		{
			name: "successful initialization",
			config: &PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "test",
				Password: "test",
				DBname:   "test",
			},
			clusterId: "test-cluster",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				// Table creation
				(*mock).ExpectExec("CREATE TABLE IF NOT EXISTS policyreports").
					WillReturnResult(sqlmock.NewResult(0, 0))
				(*mock).ExpectExec("CREATE INDEX IF NOT EXISTS policyreportnamespace").
					WillReturnResult(sqlmock.NewResult(0, 0))
				(*mock).ExpectExec("CREATE INDEX IF NOT EXISTS policyreportcluster").
					WillReturnResult(sqlmock.NewResult(0, 0))
				(*mock).ExpectExec("CREATE TABLE IF NOT EXISTS clusterpolicyreports").
					WillReturnResult(sqlmock.NewResult(0, 0))
				(*mock).ExpectExec("CREATE INDEX IF NOT EXISTS clusterpolicyreportcluster").
					WillReturnResult(sqlmock.NewResult(0, 0))
				(*mock).ExpectExec("CREATE TABLE IF NOT EXISTS ephemeralreports").
					WillReturnResult(sqlmock.NewResult(0, 0))
				(*mock).ExpectExec("CREATE INDEX IF NOT EXISTS ephemeralreportnamespace").
					WillReturnResult(sqlmock.NewResult(0, 0))
				(*mock).ExpectExec("CREATE INDEX IF NOT EXISTS ephemeralreportcluster").
					WillReturnResult(sqlmock.NewResult(0, 0))
				(*mock).ExpectExec("CREATE TABLE IF NOT EXISTS clusterephemeralreports").
					WillReturnResult(sqlmock.NewResult(0, 0))
				(*mock).ExpectExec("CREATE INDEX IF NOT EXISTS clusterephemeralreportcluster").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectedError:  nil,
			expectedStores: true,
		},
		{
			name: "primary DB connection failure",
			config: &PostgresConfig{
				Host:     "invalid-host",
				Port:     5432,
				User:     "test",
				Password: "test",
				DBname:   "test",
			},
			clusterId:     "test-cluster",
			setupMocks:    func(mock *sqlmock.Sqlmock) {},
			expectedError: errors.New("failed to open primary db"),
		},
		{
			name: "table creation failure",
			config: &PostgresConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "test",
				Password: "test",
				DBname:   "test",
			},
			clusterId: "test-cluster",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectExec("CREATE TABLE IF NOT EXISTS policyreports").
					WillReturnError(errors.New("table creation failed"))
			},
			expectedError: errors.New("failed to create table"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock DB
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Setup mock expectations
			tt.setupMocks(&mock)

			// Create storage
			storage, err := New(tt.config, tt.clusterId)

			// Verify error
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				return
			}
			assert.NoError(t, err)

			// Verify stores
			if tt.expectedStores {
				assert.NotNil(t, storage.PolicyReports())
				assert.NotNil(t, storage.ClusterPolicyReports())
				assert.NotNil(t, storage.EphemeralReports())
				assert.NotNil(t, storage.ClusterEphemeralReports())
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestPingWithRetry(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*sqlmock.Sqlmock)
		expectedError error
		expectedPings int
	}{
		{
			name: "successful ping on first try",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectPing()
			},
			expectedError: nil,
			expectedPings: 1,
		},
		{
			name: "successful ping after retries",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				// First two pings fail
				(*mock).ExpectPing().WillReturnError(errors.New("ping failed"))
				(*mock).ExpectPing().WillReturnError(errors.New("ping failed"))
				// Third ping succeeds
				(*mock).ExpectPing()
			},
			expectedError: nil,
			expectedPings: 3,
		},
		{
			name: "max retries exceeded",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				// All pings fail
				for i := 0; i < maxRetries; i++ {
					(*mock).ExpectPing().WillReturnError(errors.New("ping failed"))
				}
			},
			expectedError: errors.New("could not connect after"),
			expectedPings: maxRetries,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock DB
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Setup mock expectations
			tt.setupMocks(&mock)

			// Execute ping
			err = pingWithRetry(db)

			// Verify error
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*sqlmock.Sqlmock)
		expectedReady bool
		expectedError error
	}{
		{
			name: "DB is ready",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectPing()
			},
			expectedReady: true,
			expectedError: nil,
		},
		{
			name: "DB is not ready",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectPing().WillReturnError(errors.New("ping failed"))
			},
			expectedReady: false,
			expectedError: errors.New("ping failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock DB
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Setup mock expectations
			tt.setupMocks(&mock)

			// Create storage
			store := &postgresstore{
				db: db,
			}

			// Check readiness
			ready := store.Ready()
			assert.Equal(t, tt.expectedReady, ready)

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
