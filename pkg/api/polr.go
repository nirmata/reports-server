package api

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/kyverno/reports-server/pkg/storage"
	"github.com/kyverno/reports-server/pkg/utils"
	errorpkg "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"
	"sigs.k8s.io/wg-policy-prototypes/policy-report/pkg/api/wgpolicyk8s.io/v1alpha2"
)

type polrStore struct {
	broadcaster *watch.Broadcaster
	store       storage.Interface
}

func PolicyReportStore(store storage.Interface) API {
	broadcaster := watch.NewBroadcaster(1000, watch.WaitIfChannelFull)

	return &polrStore{
		broadcaster: broadcaster,
		store:       store,
	}
}

func (p *polrStore) New() runtime.Object {
	return &v1alpha2.PolicyReport{}
}

func (p *polrStore) Destroy() {
}

func (p *polrStore) Kind() string {
	return "PolicyReport"
}

func (p *polrStore) NewList() runtime.Object {
	return &v1alpha2.PolicyReportList{}
}

func (p *polrStore) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	var labelSelector labels.Selector
	// fieldSelector := fields.Everything() // TODO: Field selectors
	if options != nil {
		if options.LabelSelector != nil {
			labelSelector = options.LabelSelector
		}
		// if options.FieldSelector != nil {
		// 	fieldSelector = options.FieldSelector
		// }
	}
	namespace := genericapirequest.NamespaceValue(ctx)

	klog.Infof("listing policy reports for namespace=%s", namespace)
	list, err := p.listPolr(namespace)
	if err != nil {
		return nil, errors.NewBadRequest("failed to list resource policyreport")
	}

	// if labelSelector == labels.Everything() {
	// 	return list, nil
	// }

	polrList := &v1alpha2.PolicyReportList{
		Items:    make([]v1alpha2.PolicyReport, 0),
		ListMeta: metav1.ListMeta{},
	}
	var desiredRv uint64
	if len(options.ResourceVersion) == 0 {
		desiredRv = 1
	} else {
		desiredRv, err = strconv.ParseUint(options.ResourceVersion, 10, 64)
		if err != nil {
			return nil, err
		}
	}
	var resourceVersion uint64
	resourceVersion = 1
	for _, polr := range list.Items {
		allow, rv, err := allowObjectListWatch(polr.ObjectMeta, labelSelector, desiredRv, options.ResourceVersionMatch)
		if err != nil {
			return nil, err
		}
		if rv > resourceVersion {
			resourceVersion = rv
		}
		if allow {
			polrList.Items = append(polrList.Items, polr)
		}
	}
	polrList.ListMeta.ResourceVersion = strconv.FormatUint(resourceVersion, 10)
	klog.Infof("filtered list found length: %d", len(polrList.Items))
	return polrList, nil
}

func (p *polrStore) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	namespace := genericapirequest.NamespaceValue(ctx)

	klog.Infof("getting policy reports name=%s namespace=%s", name, namespace)
	report, err := p.getPolr(name, namespace)
	if err != nil || report == nil {
		return nil, errors.NewNotFound(utils.PolicyReportsGR, name)
	}
	return report, nil
}

func (p *polrStore) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	isDryRun := slices.Contains(options.DryRun, "All")

	err := createValidation(ctx, obj)
	if err != nil {
		switch options.FieldValidation {
		case "Ignore":
		case "Warn":
			// return &admissionv1.AdmissionResponse{
			// 	Allowed:  false,
			// 	Warnings: []string{err.Error()},
			// }, nil
		case "Strict":
			return nil, err
		}
	}

	polr, ok := obj.(*v1alpha2.PolicyReport)
	if !ok {
		return nil, errors.NewBadRequest("failed to validate policy report")
	}

	namespace := genericapirequest.NamespaceValue(ctx)

	if len(polr.Namespace) == 0 {
		polr.Namespace = namespace
	}
	if polr.Name == "" {
		if polr.GenerateName == "" {
			return nil, errors.NewConflict(utils.PolicyReportsGR, polr.Name, fmt.Errorf("name and generate name not provided"))
		}
		polr.Name = nameGenerator.GenerateName(polr.GenerateName)
	}

	polr.Annotations = labelReports(polr.Annotations)
	polr.Generation = 1
	klog.Infof("creating policy reports name=%s namespace=%s", polr.Name, polr.Namespace)
	if !isDryRun {
		r, err := p.createPolr(polr)
		if err != nil {
			return nil, errors.NewAlreadyExists(utils.PolicyReportsGR, polr.Name)
		}
		if err := p.broadcaster.Action(watch.Added, r); err != nil {
			klog.ErrorS(err, "failed to broadcast event")
		}
	}

	return obj, nil
}

func (p *polrStore) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	isDryRun := slices.Contains(options.DryRun, "All")
	namespace := genericapirequest.NamespaceValue(ctx)

	oldObj, err := p.getPolr(name, namespace)
	if err != nil && !forceAllowCreate {
		return nil, false, err
	}

	updatedObject, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil && !forceAllowCreate {
		return nil, false, err
	}
	polr := updatedObject.(*v1alpha2.PolicyReport)

	if forceAllowCreate {
		r, err := p.updatePolr(polr, oldObj)
		if err != nil {
			klog.ErrorS(err, "failed to update resource")
		}
		if err := p.broadcaster.Action(watch.Modified, r); err != nil {
			klog.ErrorS(err, "failed to broadcast event")
		}
		return updatedObject, true, nil
	}

	err = updateValidation(ctx, updatedObject, oldObj)
	if err != nil {
		switch options.FieldValidation {
		case "Ignore":
		case "Warn":
			// return &admissionv1.AdmissionResponse{
			// 	Allowed:  false,
			// 	Warnings: []string{err.Error()},
			// }, nil
		case "Strict":
			return nil, false, err
		}
	}

	polr, ok := updatedObject.(*v1alpha2.PolicyReport)
	if !ok {
		return nil, false, errors.NewBadRequest("failed to validate policy report")
	}

	if len(polr.Namespace) == 0 {
		polr.Namespace = namespace
	}

	polr.Annotations = labelReports(polr.Annotations)
	polr.Generation += 1
	klog.Infof("updating policy reports name=%s namespace=%s", polr.Name, polr.Namespace)
	if !isDryRun {
		r, err := p.updatePolr(polr, oldObj)
		if err != nil {
			return nil, false, errors.NewBadRequest(fmt.Sprintf("cannot create policy report: %s", err.Error()))
		}
		if err := p.broadcaster.Action(watch.Modified, r); err != nil {
			klog.ErrorS(err, "failed to broadcast event")
		}
	}

	return updatedObject, true, nil
}

func (p *polrStore) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	isDryRun := slices.Contains(options.DryRun, "All")
	namespace := genericapirequest.NamespaceValue(ctx)

	polr, err := p.getPolr(name, namespace)
	if err != nil {
		klog.ErrorS(err, "Failed to find polrs", "name", name, "namespace", klog.KRef("", namespace))
		return nil, false, errors.NewNotFound(utils.PolicyReportsGR, name)
	}

	err = deleteValidation(ctx, polr)
	if err != nil {
		klog.ErrorS(err, "invalid resource", "name", name, "namespace", klog.KRef("", namespace))
		return nil, false, errors.NewBadRequest(fmt.Sprintf("invalid resource: %s", err.Error()))
	}

	klog.Infof("deleting policy reports name=%s namespace=%s", polr.Name, polr.Namespace)
	if !isDryRun {
		err = p.deletePolr(polr)
		if err != nil {
			klog.ErrorS(err, "failed to delete polr", "name", name, "namespace", klog.KRef("", namespace))
			return nil, false, errors.NewBadRequest(fmt.Sprintf("failed to delete policyreport: %s", err.Error()))
		}
		if err := p.broadcaster.Action(watch.Deleted, polr); err != nil {
			klog.ErrorS(err, "failed to broadcast event")
		}
	}

	return polr, true, nil // TODO: Add protobuf in wgpolicygroup
}

func (p *polrStore) DeleteCollection(ctx context.Context, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions, listOptions *metainternalversion.ListOptions) (runtime.Object, error) {
	isDryRun := slices.Contains(options.DryRun, "All")
	namespace := genericapirequest.NamespaceValue(ctx)

	obj, err := p.List(ctx, listOptions)
	if err != nil {
		klog.ErrorS(err, "Failed to find polrs", "namespace", klog.KRef("", namespace))
		return nil, errors.NewBadRequest("Failed to find policy reports")
	}

	polrList, ok := obj.(*v1alpha2.PolicyReportList)
	if !ok {
		klog.ErrorS(err, "Failed to parse polrs", "namespace", klog.KRef("", namespace))
		return nil, errors.NewBadRequest("Failed to parse policy reports")
	}

	if !isDryRun {
		for _, polr := range polrList.Items {
			_, isDeleted, err := p.Delete(ctx, polr.GetName(), deleteValidation, options)
			if !isDeleted {
				klog.ErrorS(err, "Failed to delete polr", "name", polr.GetName(), "namespace", klog.KRef("", namespace))
				return nil, errors.NewBadRequest(fmt.Sprintf("Failed to delete policy report: %s/%s", polr.Namespace, polr.GetName()))
			}
		}
	}
	return polrList, nil
}

func (p *polrStore) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	klog.Infof("watching policy reports rv=%s", options.ResourceVersion)
	switch options.ResourceVersion {
	case "", "0":
		return p.broadcaster.Watch()
	default:
		break
	}
	items, err := p.List(ctx, options)
	if err != nil {
		return nil, err
	}
	list, ok := items.(*v1alpha2.PolicyReportList)
	if !ok {
		return nil, fmt.Errorf("failed to convert runtime object into policy report list")
	}
	events := make([]watch.Event, len(list.Items))
	for i, pol := range list.Items {
		report := pol.DeepCopy()
		events[i] = watch.Event{
			Type:   watch.Added,
			Object: report,
		}
	}
	return p.broadcaster.WatchWithPrefix(events)
}

func (p *polrStore) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
	var table metav1beta1.Table

	switch t := object.(type) {
	case *v1alpha2.PolicyReport:
		table.ResourceVersion = t.ResourceVersion
		table.SelfLink = t.SelfLink //nolint:staticcheck // keep deprecated field to be backward compatible
		addPolicyReportToTable(&table, *t)
	case *v1alpha2.PolicyReportList:
		table.ResourceVersion = t.ResourceVersion
		table.SelfLink = t.SelfLink //nolint:staticcheck // keep deprecated field to be backward compatible
		table.Continue = t.Continue
		addPolicyReportToTable(&table, t.Items...)
	default:
	}

	return &table, nil
}

func (p *polrStore) NamespaceScoped() bool {
	return true
}

func (p *polrStore) GetSingularName() string {
	return "policyreport"
}

func (p *polrStore) ShortNames() []string {
	return []string{"polr"}
}

func (p *polrStore) getPolr(name, namespace string) (*v1alpha2.PolicyReport, error) {
	val, err := p.store.PolicyReports().Get(context.TODO(), name, namespace)
	if err != nil {
		return nil, errorpkg.Wrapf(err, "could not find policy report in store")
	}

	return val.DeepCopy(), nil
}

func (p *polrStore) listPolr(namespace string) (*v1alpha2.PolicyReportList, error) {
	valList, err := p.store.PolicyReports().List(context.TODO(), namespace)
	if err != nil {
		return nil, errorpkg.Wrapf(err, "could not find policy report in store")
	}

	reportList := &v1alpha2.PolicyReportList{
		Items: valList,
	}

	klog.Infof("value found of length:%d", len(reportList.Items))
	return reportList, nil
}

func (p *polrStore) createPolr(report *v1alpha2.PolicyReport) (*v1alpha2.PolicyReport, error) {
	report.ResourceVersion = p.store.UseResourceVersion()
	report.UID = uuid.NewUUID()
	report.CreationTimestamp = metav1.Now()

	return report, p.store.PolicyReports().Create(context.TODO(), *report)
}

func (p *polrStore) updatePolr(report *v1alpha2.PolicyReport, _ *v1alpha2.PolicyReport) (*v1alpha2.PolicyReport, error) {
	report.ResourceVersion = p.store.UseResourceVersion()
	return report, p.store.PolicyReports().Update(context.TODO(), *report)
}

func (p *polrStore) deletePolr(report *v1alpha2.PolicyReport) error {
	return p.store.PolicyReports().Delete(context.TODO(), report.Name, report.Namespace)
}
