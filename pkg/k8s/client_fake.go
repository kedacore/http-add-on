package k8s

import (
	"context"
	"encoding/json"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ client.Client = &FakeRuntimeClient{}
var _ client.Reader = &FakeRuntimeClientReader{}
var _ client.Writer = &FakeRuntimeClientWriter{}
var _ client.StatusClient = &FakeRuntimeStatusClient{}

// FakeRuntimeClient is a fake implementation of
// (k8s.io/controller-runtime/pkg/client).Client
type FakeRuntimeClient struct {
	*FakeRuntimeClientReader
	*FakeRuntimeClientWriter
	*FakeRuntimeStatusClient
}

func NewFakeRuntimeClient() *FakeRuntimeClient {
	return &FakeRuntimeClient{
		FakeRuntimeClientReader: &FakeRuntimeClientReader{},
		FakeRuntimeClientWriter: &FakeRuntimeClientWriter{},
		FakeRuntimeStatusClient: &FakeRuntimeStatusClient{},
	}
}

// Scheme implements the controller-runtime Client interface.
//
// NOTE: this method is not implemented and always returns nil.
func (f *FakeRuntimeStatusClient) Scheme() *runtime.Scheme {
	return nil
}

// RESTMapper implements the controller-runtime Client interface.
//
// NOTE: this method is not implemented and always returns nil.
func (f *FakeRuntimeClientReader) RESTMapper() meta.RESTMapper {
	return nil
}

type GetCall struct {
	Key client.ObjectKey
	Obj client.Object
}

// FakeRuntimeClientReader is a fake implementation of
// (k8s.io/controller-runtime/pkg/client).ClientReader
type FakeRuntimeClientReader struct {
	GetCalls  []GetCall
	GetFunc   func() client.Object
	ListCalls []client.ObjectList
	ListFunc  func() client.ObjectList
}

func (f *FakeRuntimeClientReader) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
) error {
	f.GetCalls = append(f.GetCalls, GetCall{
		Key: key,
		Obj: obj,
	})
	// marshal the GetFunc return value, then unmarshal
	// it back into the obj parameter.
	b, err := json.Marshal(f.GetFunc())
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, obj); err != nil {
		return err
	}

	return nil
}

func (f *FakeRuntimeClientReader) List(
	ctx context.Context,
	list client.ObjectList,
	opts ...client.ListOption,
) error {
	f.ListCalls = append(f.ListCalls, list)
	b, err := json.Marshal(f.ListFunc())
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, list); err != nil {
		return err
	}
	return nil
}

// FakeRuntimeClientWriter is a fake implementation of
// (k8s.io/controller-runtime/pkg/client).ClientWriter
//
// It stores all method calls in the respective struct
// fields. Instances of FakeRuntimeClientWriter are not
// concurrency-safe
type FakeRuntimeClientWriter struct {
	Creates      []client.Object
	Deletes      []client.Object
	Updates      []client.Object
	Patches      []client.Object
	DeleteAllOfs []client.Object
}

func (f *FakeRuntimeClientWriter) Create(
	ctx context.Context,
	obj client.Object,
	opts ...client.CreateOption,
) error {
	f.Creates = append(f.Creates, obj)
	return nil
}

func (f *FakeRuntimeClientWriter) Delete(
	ctx context.Context,
	obj client.Object,
	opts ...client.DeleteOption,
) error {
	f.Deletes = append(f.Deletes, obj)
	return nil
}

func (f *FakeRuntimeClientWriter) Update(
	ctx context.Context,
	obj client.Object,
	opts ...client.UpdateOption,
) error {
	f.Updates = append(f.Updates, obj)
	return nil
}

func (f *FakeRuntimeClientWriter) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	f.Patches = append(f.Patches, obj)
	return nil
}

func (f *FakeRuntimeClientWriter) DeleteAllOf(
	ctx context.Context,
	obj client.Object,
	opts ...client.DeleteAllOfOption,
) error {
	f.DeleteAllOfs = append(f.DeleteAllOfs, obj)
	return nil
}

// FakeRuntimeStatusClient is a fake implementation of
// (k8s.io/controller-runtime/pkg/client).StatusClient
type FakeRuntimeStatusClient struct {
}

// Status implements the controller-runtime StatusClient
// interface.
//
// NOTE: this function isn't implemented and always returns
// nil.
func (f *FakeRuntimeStatusClient) Status() client.StatusWriter {
	return nil
}
