package rest

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog/v2"

	"github.com/grafana/grafana/pkg/apimachinery/utils"
)

type dualWriteContextKey struct{}

func IsDualWriteUpdate(ctx context.Context) bool {
	return ctx.Value(dualWriteContextKey{}) == true
}

type DualWriterMode2 struct {
	Storage Storage
	Legacy  LegacyStorage
	*dualWriterMetrics
	resource string
	Log      klog.Logger
}

const mode2Str = "2"

// newDualWriterMode2 returns a new DualWriter in mode 2.
// Mode 2 represents writing to LegacyStorage first, then to Storage.
// When reading, values from LegacyStorage will be returned.
func newDualWriterMode2(legacy LegacyStorage, storage Storage, dwm *dualWriterMetrics, resource string) *DualWriterMode2 {
	return &DualWriterMode2{
		Legacy:            legacy,
		Storage:           storage,
		Log:               klog.NewKlogr().WithName("DualWriterMode2").WithValues("mode", mode2Str, "resource", resource),
		dualWriterMetrics: dwm,
		resource:          resource,
	}
}

// Mode returns the mode of the dual writer.
func (d *DualWriterMode2) Mode() DualWriterMode {
	return Mode2
}

// Create overrides the behavior of the generic DualWriter and writes to LegacyStorage and Storage.
func (d *DualWriterMode2) Create(ctx context.Context, in runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	var method = "create"
	log := d.Log.WithValues("method", method)
	ctx = klog.NewContext(ctx, log)

	accIn, err := meta.Accessor(in)
	if err != nil {
		return nil, err
	}

	if accIn.GetUID() != "" {
		return nil, fmt.Errorf("UID should be empty: %v", accIn.GetUID())
	}

	startLegacy := time.Now()
	createdFromLegacy, err := d.Legacy.Create(ctx, in, createValidation, options)
	if err != nil {
		log.Error(err, "unable to create object in legacy storage")
		d.recordLegacyDuration(true, mode2Str, d.resource, method, startLegacy)
		return nil, err
	}
	d.recordLegacyDuration(false, mode2Str, d.resource, method, startLegacy)

	createdCopy := createdFromLegacy.DeepCopyObject()

	accCreated, err := meta.Accessor(createdCopy)
	if err != nil {
		return nil, err
	}

	accCreated.SetResourceVersion("")

	startStorage := time.Now()
	createdFromStorage, err := d.Storage.Create(ctx, createdCopy, createValidation, options)
	if err != nil {
		log.WithValues("name").Error(err, "unable to create object in storage")
		d.recordStorageDuration(true, mode2Str, d.resource, method, startStorage)
		return createdFromStorage, err
	}
	d.recordStorageDuration(false, mode2Str, d.resource, method, startStorage)

	go func() {
		areEqual := Compare(createdFromStorage, createdFromLegacy)
		d.recordOutcome(mode2Str, getName(createdFromStorage), areEqual, method)
		if !areEqual {
			log.Info("object from legacy and storage are not equal")
		}
	}()

	return createdFromLegacy, err
}

// Get retrieves an object from Storage if possible, and if not it falls back to LegacyStorage.
func (d *DualWriterMode2) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	var method = "get"
	log := d.Log.WithValues("name", name, "resourceVersion", options.ResourceVersion, "method", method)
	ctx = klog.NewContext(ctx, log)

	startLegacy := time.Now()
	objLegacy, err := d.Legacy.Get(ctx, name, options)
	if err != nil {
		log.Error(err, "unable to fetch object from legacy")
		d.recordLegacyDuration(true, mode2Str, d.resource, method, startLegacy)
		return nil, err
	}
	d.recordLegacyDuration(false, mode2Str, d.resource, method, startLegacy)

	startStorage := time.Now()
	objStorage, err := d.Storage.Get(ctx, name, options)
	d.recordStorageDuration(err != nil, mode2Str, d.resource, method, startStorage)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "unable to fetch object from storage")
			return nil, err
		}
		log.Info("object not found in storage, dual write or migration didn't happen yet")
	}

	go func() {
		areEqual := Compare(objStorage, objLegacy)
		d.recordOutcome(mode2Str, name, areEqual, method)
		if !areEqual {
			log.Info("object from legacy and storage are not equal")
		}
	}()

	return objLegacy, nil
}

// List overrides the behavior of the generic DualWriter.
func (d *DualWriterMode2) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	var method = "list"
	log := d.Log.WithValues("resourceVersion", options.ResourceVersion, "method", method)
	ctx = klog.NewContext(ctx, log)

	startLegacy := time.Now()
	ll, err := d.Legacy.List(ctx, options)
	if err != nil {
		log.Error(err, "unable to list objects from legacy storage")
		d.recordLegacyDuration(true, mode2Str, d.resource, method, startLegacy)
		return ll, err
	}
	d.recordLegacyDuration(false, mode2Str, d.resource, method, startLegacy)

	// Even if we don't compare, we want to fetch from unified storage and check that it doesn't error.
	startStorage := time.Now()
	if _, err := d.Storage.List(ctx, options); err != nil {
		log.Error(err, "unable to list objects from storage")
		d.recordStorageDuration(true, mode2Str, d.resource, method, startStorage)
		return nil, err
	}
	d.recordStorageDuration(false, mode2Str, d.resource, method, startStorage)

	return ll, nil
}

// DeleteCollection overrides the behavior of the generic DualWriter and deletes from both LegacyStorage and Storage.
func (d *DualWriterMode2) DeleteCollection(ctx context.Context, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions, listOptions *metainternalversion.ListOptions) (runtime.Object, error) {
	var method = "delete-collection"
	log := d.Log.WithValues("resourceVersion", listOptions.ResourceVersion, "method", method)
	ctx = klog.NewContext(ctx, log)

	startLegacy := time.Now()
	deletedLegacy, err := d.Legacy.DeleteCollection(ctx, deleteValidation, options, listOptions)
	if err != nil {
		log.WithValues("deleted", deletedLegacy).Error(err, "failed to delete collection successfully from legacy storage")
		d.recordLegacyDuration(true, mode2Str, d.resource, method, startLegacy)
		return nil, err
	}
	d.recordLegacyDuration(false, mode2Str, d.resource, method, startLegacy)

	startStorage := time.Now()
	deletedStorage, err := d.Storage.DeleteCollection(ctx, deleteValidation, options, listOptions)
	if err != nil {
		log.WithValues("deleted", deletedStorage).Error(err, "failed to delete collection successfully from Storage")
		d.recordStorageDuration(true, mode2Str, d.resource, method, startStorage)
		return nil, err
	}
	d.recordStorageDuration(false, mode2Str, d.resource, method, startStorage)

	go func() {
		areEqual := Compare(deletedStorage, deletedLegacy)
		d.recordOutcome(mode2Str, getName(deletedStorage), areEqual, method)
		if !areEqual {
			log.Info("object from legacy and storage are not equal")
		}
	}()

	return deletedLegacy, err
}

func (d *DualWriterMode2) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	var method = "delete"
	log := d.Log.WithValues("name", name, "method", method)
	ctx = klog.NewContext(ctx, log)

	// We should delete from Unified storage first so we can retry if legacy fails.
	startStorage := time.Now()
	deletedS, _, err := d.Storage.Delete(ctx, name, deleteValidation, options)
	d.recordStorageDuration(err != nil, mode2Str, d.resource, method, startStorage)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.WithValues("objectList", deletedS).Error(err, "could not delete from unified storage")
			return nil, false, err
		}
	}

	startLegacy := time.Now()
	deletedLS, async, err := d.Legacy.Delete(ctx, name, deleteValidation, options)
	d.recordLegacyDuration(err != nil, mode2Str, d.resource, method, startLegacy)
	// Deleting from legacy should always work in mode two, as legacy is still the primary database and
	// needs to have all the data.
	if err != nil {
		return nil, false, err
	}

	go func() {
		areEqual := Compare(deletedS, deletedLS)
		d.recordOutcome(mode2Str, name, areEqual, method)
		if !areEqual {
			log.WithValues("name", name).Info("object from legacy and storage are not equal")
		}
	}()

	return deletedLS, async, err
}

// Update overrides the generic behavior of the Storage and writes first to the legacy storage and then to storage.
func (d *DualWriterMode2) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	var method = "update"
	log := d.Log.WithValues("name", name, "method", method)
	ctx = klog.NewContext(ctx, log)

	// The incoming RV is not stable -- it may be from legacy or storage!
	// This sets a flag in the context and our apistore is more lenient when it exists
	ctx = context.WithValue(ctx, dualWriteContextKey{}, true)

	startLegacy := time.Now()
	objFromLegacy, created, err := d.Legacy.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
	if err != nil {
		log.WithValues("object", objFromLegacy).Error(err, "could not update in legacy storage")
		d.recordLegacyDuration(true, mode2Str, d.resource, "update", startLegacy)
		return objFromLegacy, created, err
	}
	d.recordLegacyDuration(false, mode2Str, d.resource, "update", startLegacy)

	startStorage := time.Now()
	objFromStorage, created, err := d.Storage.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
	if err != nil {
		log.WithValues("object", objFromStorage).Error(err, "could not update in storage")
		d.recordStorageDuration(true, mode2Str, d.resource, "update", startStorage)
		return objFromStorage, created, err
	}

	go func() {
		areEqual := Compare(objFromStorage, objFromLegacy)
		d.recordOutcome(mode2Str, name, areEqual, method)
		if !areEqual {
			log.WithValues("name", name).Info("object from legacy and storage are not equal")
		}
	}()

	return objFromLegacy, created, err
}

func (d *DualWriterMode2) Destroy() {
	d.Storage.Destroy()
	d.Legacy.Destroy()
}

func (d *DualWriterMode2) GetSingularName() string {
	return d.Storage.GetSingularName()
}

func (d *DualWriterMode2) NamespaceScoped() bool {
	return d.Storage.NamespaceScoped()
}

func (d *DualWriterMode2) New() runtime.Object {
	return d.Storage.New()
}

func (d *DualWriterMode2) NewList() runtime.Object {
	return d.Storage.NewList()
}

func (d *DualWriterMode2) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	return d.Storage.ConvertToTable(ctx, object, tableOptions)
}

func parseList(legacyList []runtime.Object) (map[string]int, error) {
	indexMap := map[string]int{}

	for i, obj := range legacyList {
		accessor, err := utils.MetaAccessor(obj)
		if err != nil {
			return nil, err
		}
		indexMap[accessor.GetName()] = i
	}
	return indexMap, nil
}
