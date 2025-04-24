package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/wg-policy-prototypes/policy-report/pkg/api/wgpolicyk8s.io/v1alpha2"
)

func TestClusterPolicyReportStore(t *testing.T) {
	// Sample cluster policy report for testing
	sampleReport := &v1alpha2.ClusterPolicyReport{
		Name: "test-report",
	}

	tests := []struct {
		name           string
		operation      string
		setupMocks     func(*sqlmock.Sqlmock)
		report         *v1alpha2.ClusterPolicyReport
		expectedError  error
		expectedResult interface{}
	}{
		{
			name:      "successful list all reports",
			operation: "list",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				reportJSON, _ := json.Marshal(sampleReport)
				rows := sqlmock.NewRows([]string{"report"}).AddRow(string(reportJSON))
				(*mock).ExpectQuery("SELECT report FROM clusterpolicyreports WHERE clusterId = \\$1").
					WithArgs("test-cluster").
					WillReturnRows(rows)
			},
			expectedError:  nil,
			expectedResult: []*v1alpha2.ClusterPolicyReport{sampleReport},
		},
		{
			name:      "list with no reports",
			operation: "list",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"report"})
				(*mock).ExpectQuery("SELECT report FROM clusterpolicyreports WHERE clusterId = \\$1").
					WithArgs("test-cluster").
					WillReturnRows(rows)
			},
			expectedError:  nil,
			expectedResult: []*v1alpha2.ClusterPolicyReport{},
		},
		{
			name:      "successful get report",
			operation: "get",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				reportJSON, _ := json.Marshal(sampleReport)
				rows := sqlmock.NewRows([]string{"report"}).AddRow(string(reportJSON))
				(*mock).ExpectQuery("SELECT report FROM clusterpolicyreports WHERE \\(name = \\$1\\) AND \\(clusterId = \\$2\\)").
					WithArgs("test-report", "test-cluster").
					WillReturnRows(rows)
			},
			report:         sampleReport,
			expectedError:  nil,
			expectedResult: sampleReport,
		},
		{
			name:      "get non-existent report",
			operation: "get",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectQuery("SELECT report FROM clusterpolicyreports WHERE \\(name = \\$1\\) AND \\(clusterId = \\$2\\)").
					WithArgs("non-existent", "test-cluster").
					WillReturnError(sql.ErrNoRows)
			},
			report: &v1alpha2.ClusterPolicyReport{
				Name: "non-existent",
			},
			expectedError: errors.New("no such policy report"),
		},
		{
			name:      "successful create report",
			operation: "create",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				reportJSON, _ := json.Marshal(sampleReport)
				(*mock).ExpectExec("INSERT INTO clusterpolicyreports \\(name, report, clusterId\\) VALUES \\(\\$1, \\$2, \\$3\\)").
					WithArgs(sampleReport.Name, string(reportJSON), "test-cluster").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			report:        sampleReport,
			expectedError: nil,
		},
		{
			name:      "create duplicate report",
			operation: "create",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				reportJSON, _ := json.Marshal(sampleReport)
				(*mock).ExpectExec("INSERT INTO clusterpolicyreports \\(name, report, clusterId\\) VALUES \\(\\$1, \\$2, \\$3\\)").
					WithArgs(sampleReport.Name, string(reportJSON), "test-cluster").
					WillReturnError(errors.New("duplicate key"))
			},
			report:        sampleReport,
			expectedError: errors.New("duplicate key"),
		},
		{
			name:      "successful update report",
			operation: "update",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				reportJSON, _ := json.Marshal(sampleReport)
				(*mock).ExpectExec("UPDATE clusterpolicyreports SET report = \\$1 WHERE \\(name = \\$2\\) AND \\(clusterId = \\$3\\)").
					WithArgs(string(reportJSON), sampleReport.Name, "test-cluster").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			report:        sampleReport,
			expectedError: nil,
		},
		{
			name:      "update non-existent report",
			operation: "update",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				reportJSON, _ := json.Marshal(sampleReport)
				(*mock).ExpectExec("UPDATE clusterpolicyreports SET report = \\$1 WHERE \\(name = \\$2\\) AND \\(clusterId = \\$3\\)").
					WithArgs(string(reportJSON), sampleReport.Name, "test-cluster").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			report:        sampleReport,
			expectedError: nil, // Update of non-existent report is not an error
		},
		{
			name:      "successful delete report",
			operation: "delete",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectExec("DELETE FROM clusterpolicyreports WHERE \\(name = \\$1\\) AND \\(clusterId = \\$2\\)").
					WithArgs("test-report", "test-cluster").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			report:        sampleReport,
			expectedError: nil,
		},
		{
			name:      "delete non-existent report",
			operation: "delete",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectExec("DELETE FROM clusterpolicyreports WHERE \\(name = \\$1\\) AND \\(clusterId = \\$2\\)").
					WithArgs("non-existent", "test-cluster").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			report: &v1alpha2.ClusterPolicyReport{
				Name: "non-existent",
			},
			expectedError: nil, // Delete of non-existent report is not an error
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

			// Create store
			store := &cpolrdb{
				MultiDB:   &MultiDB{PrimaryDB: db},
				clusterId: "test-cluster",
			}

			// Execute operation
			var result interface{}
			var err error

			switch tt.operation {
			case "list":
				result, err = store.List(context.Background())
			case "get":
				result, err = store.Get(context.Background(), tt.report.Name)
			case "create":
				err = store.Create(context.Background(), tt.report)
			case "update":
				err = store.Update(context.Background(), tt.report)
			case "delete":
				err = store.Delete(context.Background(), tt.report.Name)
			}

			// Verify error
			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				return
			}
			assert.NoError(t, err)

			// Verify result
			if tt.expectedResult != nil {
				assert.Equal(t, tt.expectedResult, result)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
