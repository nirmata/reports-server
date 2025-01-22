package utils

import (
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	v2 "k8s.io/api/autoscaling/v2"
)

var (
	EphemeralReportsGR        = reportsv1.Resource("ephemeralreports")
	ClusterEphemeralReportsGR = reportsv1.Resource("clusterephemeralreports")
	PolicyReportsGR           = v2.Resource("policyreports")
	ClusterPolicyReportsGR    = v2.Resource("clusterephemeralreports")

	EphemeralReportsGVK        = reportsv1.SchemeGroupVersion.WithKind("EphemeralReport")
	ClusterEphemeralReportsGVK = reportsv1.SchemeGroupVersion.WithKind("ClusterEphemeralReport")
	PolicyReportsGVK           = v2.SchemeGroupVersion.WithKind("PolicyReport")
	ClusterPolicyReportsGVK    = v2.SchemeGroupVersion.WithKind("ClusterEphemeralReport")
)
