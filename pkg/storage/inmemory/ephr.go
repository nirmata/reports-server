package inmemory

import (
	"context"
	"fmt"
	"strings"
	"sync"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	metrics "github.com/kyverno/reports-server/pkg/storage/metrics"
	"github.com/kyverno/reports-server/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
)

type ephrdb struct {
	sync.Mutex
	db *db[reportsv1.EphemeralReport]
}

func (e *ephrdb) key(name, namespace string) string {
	return fmt.Sprintf("ephr/%s/%s", namespace, name)
}

func (e *ephrdb) List(ctx context.Context, namespace string) ([]*reportsv1.EphemeralReport, error) {
	e.Lock()
	defer e.Unlock()

	klog.Infof("listing all values for namespace:%s", namespace)
	res := make([]*reportsv1.EphemeralReport, 0)

	for _, k := range e.db.Keys() {
		if namespace == "" || strings.HasPrefix(k, fmt.Sprintf("ephr/%s/", namespace)) {
			v, _ := e.db.Get(k)
			res = append(res, v)
			klog.Infof("value found for prefix:%s, key:%s", namespace, k)
		}
	}

	klog.Infof("list found length: %d", len(res))
	return res, nil
}

func (e *ephrdb) Get(ctx context.Context, name, namespace string) (*reportsv1.EphemeralReport, error) {
	e.Lock()
	defer e.Unlock()

	key := e.key(name, namespace)
	klog.Infof("getting value for key:%s", key)
	if val, _ := e.db.Get(key); val != nil {
		klog.Infof("value found for key:%s", key)
		return val, nil
	} else {
		klog.Errorf("value not found for key:%s", key)
		return nil, errors.NewNotFound(utils.EphemeralReportsGR, key)
	}
}

func (e *ephrdb) Create(ctx context.Context, ephr *reportsv1.EphemeralReport) error {
	e.Lock()
	defer e.Unlock()

	key := e.key(ephr.Name, ephr.Namespace)
	klog.Infof("creating entry for key:%s", key)
	if val, _ := e.db.Get(key); val != nil {
		klog.Errorf("entry already exists k:%s", key)
		return errors.NewAlreadyExists(utils.EphemeralReportsGR, key)
	} else {
		klog.Infof("entry created for key:%s", key)
		metrics.UpdatePolicyReportMetrics("etcd", "create", ephr, false)
		return e.db.Store(key, *ephr)
	}
}

func (e *ephrdb) Update(ctx context.Context, ephr *reportsv1.EphemeralReport) error {
	e.Lock()
	defer e.Unlock()

	key := e.key(ephr.Name, ephr.Namespace)
	klog.Infof("updating entry for key:%s", key)
	if val, _ := e.db.Get(key); val == nil {
		klog.Errorf("entry does not exist k:%s", key)
		return errors.NewNotFound(utils.EphemeralReportsGR, key)
	} else {
		klog.Infof("entry updated for key:%s", key)
		metrics.UpdatePolicyReportMetrics("etcd", "update", ephr, false)
		return e.db.Store(key, *ephr)
	}
}

func (e *ephrdb) Delete(ctx context.Context, name, namespace string) error {
	e.Lock()
	defer e.Unlock()

	key := e.key(name, namespace)
	klog.Infof("deleting entry for key:%s", key)
	if val, _ := e.db.Get(key); val == nil {
		klog.Errorf("entry does not exist k:%s", key)
		return errors.NewNotFound(utils.EphemeralReportsGR, key)
	} else {
		report := reportsv1.EphemeralReport{}
		e.db.Delete(key)
		klog.Infof("entry deleted for key:%s", key)
		metrics.UpdatePolicyReportMetrics("etcd", "delete", report, false)
		return nil
	}
}
