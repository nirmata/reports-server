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

func TestPolicyReportStore(t *testing.T) {
	// Sample policy report for testing
	sampleReport := &v1alpha2.PolicyReport{
		Name:      "test-report",
		Namespace: "test-namespace",
	}

	tests := []struct {
		name           string
		operation      string
		setupMocks     func(*sqlmock.Sqlmock)
		report         *v1alpha2.PolicyReport
		namespace      string
		expectedError  error
		expectedResult interface{}
	}{
		{
			name:      "successful list all reports",
			operation: "list",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				reportJSON, _ := json.Marshal(sampleReport)
				rows := sqlmock.NewRows([]string{"report"}).AddRow(string(reportJSON))
				(*mock).ExpectQuery("SELECT report FROM policyreports WHERE clusterId = \\$1").
					WithArgs("test-cluster").
					WillReturnRows(rows)
			},
			namespace:      "",
			expectedError:  nil,
			expectedResult: []*v1alpha2.PolicyReport{sampleReport},
		},
		{
			name:      "successful list namespace reports",
			operation: "list",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				reportJSON, _ := json.Marshal(sampleReport)
				rows := sqlmock.NewRows([]string{"report"}).AddRow(string(reportJSON))
				(*mock).ExpectQuery("SELECT report FROM policyreports WHERE namespace = \\$1 AND clusterId = \\$2").
					WithArgs("test-namespace", "test-cluster").
					WillReturnRows(rows)
			},
			namespace:      "test-namespace",
			expectedError:  nil,
			expectedResult: []*v1alpha2.PolicyReport{sampleReport},
		},
		{
			name:      "list with no reports",
			operation: "list",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"report"})
				(*mock).ExpectQuery("SELECT report FROM policyreports WHERE clusterId = \\$1").
					WithArgs("test-cluster").
					WillReturnRows(rows)
			},
			namespace:      "",
			expectedError:  nil,
			expectedResult: []*v1alpha2.PolicyReport{},
		},
		{
			name:      "successful get report",
			operation: "get",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				reportJSON, _ := json.Marshal(sampleReport)
				rows := sqlmock.NewRows([]string{"report"}).AddRow(string(reportJSON))
				(*mock).ExpectQuery("SELECT report FROM policyreports WHERE \\(namespace = \\$1\\) AND \\(name = \\$2\\) AND \\(clusterId = \\$3\\)").
					WithArgs("test-namespace", "test-report", "test-cluster").
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
				(*mock).ExpectQuery("SELECT report FROM policyreports WHERE \\(namespace = \\$1\\) AND \\(name = \\$2\\) AND \\(clusterId = \\$3\\)").
					WithArgs("test-namespace", "non-existent", "test-cluster").
					WillReturnError(sql.ErrNoRows)
			},
			report: &v1alpha2.PolicyReport{
				Name:      "non-existent",
				Namespace: "test-namespace",
			},
			expectedError: errors.New("no such policy report"),
		},
		{
			name:      "successful create report",
			operation: "create",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				reportJSON, _ := json.Marshal(sampleReport)
				(*mock).ExpectExec("INSERT INTO policyreports \\(name, namespace, report, clusterId\\) VALUES \\(\\$1, \\$2, \\$3, \\$4\\)").
					WithArgs(sampleReport.Name, sampleReport.Namespace, string(reportJSON), "test-cluster").
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
				(*mock).ExpectExec("INSERT INTO policyreports \\(name, namespace, report, clusterId\\) VALUES \\(\\$1, \\$2, \\$3, \\$4\\)").
					WithArgs(sampleReport.Name, sampleReport.Namespace, string(reportJSON), "test-cluster").
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
				(*mock).ExpectExec("UPDATE policyreports SET report = \\$1 WHERE \\(namespace = \\$2\\) AND \\(name = \\$3\\) AND \\(clusterId = \\$4\\)").
					WithArgs(string(reportJSON), sampleReport.Namespace, sampleReport.Name, "test-cluster").
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
				(*mock).ExpectExec("UPDATE policyreports SET report = \\$1 WHERE \\(namespace = \\$2\\) AND \\(name = \\$3\\) AND \\(clusterId = \\$4\\)").
					WithArgs(string(reportJSON), sampleReport.Namespace, sampleReport.Name, "test-cluster").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			report:        sampleReport,
			expectedError: nil, // Update of non-existent report is not an error
		},
		{
			name:      "successful delete report",
			operation: "delete",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectExec("DELETE FROM policyreports WHERE \\(namespace = \\$1\\) AND \\(name = \\$2\\) AND \\(clusterId = \\$3\\)").
					WithArgs("test-namespace", "test-report", "test-cluster").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			report:        sampleReport,
			expectedError: nil,
		},
		{
			name:      "delete non-existent report",
			operation: "delete",
			setupMocks: func(mock *sqlmock.Sqlmock) {
				(*mock).ExpectExec("DELETE FROM policyreports WHERE \\(namespace = \\$1\\) AND \\(name = \\$2\\) AND \\(clusterId = \\$3\\)").
					WithArgs("test-namespace", "non-existent", "test-cluster").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			report: &v1alpha2.PolicyReport{
				Name:      "non-existent",
				Namespace: "test-namespace",
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
			store := &polrdb{
				MultiDB:   &MultiDB{PrimaryDB: db},
				clusterId: "test-cluster",
			}

			// Execute operation
			var result interface{}
			var err error

			switch tt.operation {
			case "list":
				result, err = store.List(context.Background(), tt.namespace)
			case "get":
				result, err = store.Get(context.Background(), tt.report.Name, tt.report.Namespace)
			case "create":
				err = store.Create(context.Background(), tt.report)
			case "update":
				err = store.Update(context.Background(), tt.report)
			case "delete":
				err = store.Delete(context.Background(), tt.report.Name, tt.report.Namespace)
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
