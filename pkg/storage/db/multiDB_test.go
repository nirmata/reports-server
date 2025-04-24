package db

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiDB_ReadQuery(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*sqlmock.Sqlmock)
		query         string
		args          []interface{}
		expectedError error
		expectedRows  int
		usePrimary    bool
	}{
		{
			name: "successful read from replica",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectQuery("SELECT 1").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			query:         "SELECT 1",
			expectedRows:  1,
			expectedError: nil,
		},
		{
			name: "replica failure with primary fallback",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				// First replica fails
				(*mock).ExpectQuery("SELECT 1").
					WillReturnError(errors.New("replica error"))
				// Primary succeeds
				(*mock).ExpectQuery("SELECT 1").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			query:         "SELECT 1",
			expectedRows:  1,
			expectedError: nil,
			usePrimary:    true,
		},
		{
			name: "all replicas fail",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				// All replicas fail
				(*mock).ExpectQuery("SELECT 1").
					WillReturnError(errors.New("replica error"))
				// Primary fails too
				(*mock).ExpectQuery("SELECT 1").
					WillReturnError(errors.New("primary error"))
			},
			query:         "SELECT 1",
			expectedError: errors.New("primary error"),
		},
		{
			name: "no replicas available",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				// Primary succeeds
				(*mock).ExpectQuery("SELECT 1").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			query:         "SELECT 1",
			expectedRows:  1,
			expectedError: nil,
			usePrimary:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock DBs
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Setup mock expectations
			tt.setupMocks(&mock)

			// Create MultiDB instance
			multiDB := &MultiDB{
				PrimaryDB: db,
			}

			if !tt.usePrimary {
				// Add a replica
				replicaDB, replicaMock, err := sqlmock.New()
				require.NoError(t, err)
				defer replicaDB.Close()

				tt.setupMocks(&replicaMock)
				multiDB.ReadReplicaDBs = []*sql.DB{replicaDB}
			}

			// Execute query
			rows, err := multiDB.ReadQuery(context.Background(), tt.query, tt.args...)

			// Verify error
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				return
			}
			assert.NoError(t, err)
			defer rows.Close()

			// Verify rows
			count := 0
			for rows.Next() {
				count++
			}
			assert.Equal(t, tt.expectedRows, count)

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestMultiDB_ReadQueryRow(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*sqlmock.Sqlmock)
		query         string
		args          []interface{}
		expectedError error
		expectedValue int
		usePrimary    bool
	}{
		{
			name: "successful read from replica",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectQuery("SELECT 1").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			query:         "SELECT 1",
			expectedValue: 1,
			expectedError: nil,
		},
		{
			name: "replica failure with primary fallback",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				// First replica fails
				(*mock).ExpectQuery("SELECT 1").
					WillReturnError(errors.New("replica error"))
				// Primary succeeds
				(*mock).ExpectQuery("SELECT 1").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			query:         "SELECT 1",
			expectedValue: 1,
			expectedError: nil,
			usePrimary:    true,
		},
		{
			name: "all replicas fail",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				// All replicas fail
				(*mock).ExpectQuery("SELECT 1").
					WillReturnError(errors.New("replica error"))
				// Primary fails too
				(*mock).ExpectQuery("SELECT 1").
					WillReturnError(errors.New("primary error"))
			},
			query:         "SELECT 1",
			expectedError: errors.New("primary error"),
		},
		{
			name: "no replicas available",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				// Primary succeeds
				(*mock).ExpectQuery("SELECT 1").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			query:         "SELECT 1",
			expectedValue: 1,
			expectedError: nil,
			usePrimary:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock DBs
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			// Setup mock expectations
			tt.setupMocks(&mock)

			// Create MultiDB instance
			multiDB := &MultiDB{
				PrimaryDB: db,
			}

			if !tt.usePrimary {
				// Add a replica
				replicaDB, replicaMock, err := sqlmock.New()
				require.NoError(t, err)
				defer replicaDB.Close()

				tt.setupMocks(&replicaMock)
				multiDB.ReadReplicaDBs = []*sql.DB{replicaDB}
			}

			// Execute query
			row := multiDB.ReadQueryRow(context.Background(), tt.query, tt.args...)

			// Verify result
			if tt.expectedError != nil {
				err := row.Scan(nil)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				return
			}

			var value int
			err = row.Scan(&value)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValue, value)

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestMultiDB_ConcurrentAccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Setup mock expectations for concurrent access
	mock.ExpectQuery("SELECT 1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	multiDB := &MultiDB{
		PrimaryDB: db,
	}

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			rows, err := multiDB.ReadQuery(context.Background(), "SELECT 1")
			assert.NoError(t, err)
			defer rows.Close()

			for rows.Next() {
				var id int
				err := rows.Scan(&id)
				assert.NoError(t, err)
				assert.Equal(t, 1, id)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all expectations were met
	assert.NoError(t, mock.ExpectationsWereMet())
}
