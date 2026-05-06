package http

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kedacore/http-add-on/operator/controllers/http/config"
)

func TestOrphanAnnotationRemovesOwnerReference(t *testing.T) {
	r := require.New(t)
	testInfra := newCommonTestInfra("testns", "testapp")
	reconciler := &HTTPScaledObjectReconciler{
		Client:               testInfra.cl,
		Scheme:               testInfra.cl.Scheme(),
		ExternalScalerConfig: config.ExternalScaler{},
		BaseConfig:           config.Base{},
	}

	// First reconcile without annotation — ScaledObject created with owner reference
	err := reconciler.createOrUpdateApplicationResources(
		testInfra.ctx, testInfra.logger, testInfra.cl,
		config.Base{}, config.ExternalScaler{}, &testInfra.httpso,
	)
	r.NoError(err)

	so, err := getSO(testInfra.ctx, testInfra.cl, testInfra.httpso)
	r.NoError(err)
	r.Len(so.OwnerReferences, 1)

	// Add orphan annotation and reconcile again
	testInfra.httpso.Annotations[OrphanScaledObjectAnnotation] = "true"
	err = reconciler.createOrUpdateApplicationResources(
		testInfra.ctx, testInfra.logger, testInfra.cl,
		config.Base{}, config.ExternalScaler{}, &testInfra.httpso,
	)
	r.NoError(err)

	// ScaledObject must still exist but with no owner references
	so, err = getSO(testInfra.ctx, testInfra.cl, testInfra.httpso)
	r.NoError(err)
	r.Empty(so.OwnerReferences)

	// Third reconcile with orphan annotation still set — must be idempotent
	err = reconciler.createOrUpdateApplicationResources(
		testInfra.ctx, testInfra.logger, testInfra.cl,
		config.Base{}, config.ExternalScaler{}, &testInfra.httpso,
	)
	r.NoError(err)
}

func TestOrphanAnnotationWhenNoScaledObjectExists(t *testing.T) {
	r := require.New(t)
	testInfra := newCommonTestInfra("testns", "testapp")
	testInfra.httpso.Annotations[OrphanScaledObjectAnnotation] = "true"
	reconciler := &HTTPScaledObjectReconciler{
		Client:               testInfra.cl,
		Scheme:               testInfra.cl.Scheme(),
		ExternalScalerConfig: config.ExternalScaler{},
		BaseConfig:           config.Base{},
	}

	err := reconciler.createOrUpdateApplicationResources(
		testInfra.ctx, testInfra.logger, testInfra.cl,
		config.Base{}, config.ExternalScaler{}, &testInfra.httpso,
	)
	r.NoError(err)

	// ScaledObject must not have been created
	_, err = getSO(testInfra.ctx, testInfra.cl, testInfra.httpso)
	r.Error(err)
}

func TestHttpScaledObjectControllerWhenSkipAnnotationNotSet(t *testing.T) {
	r := require.New(t)

	testInfra := newCommonTestInfra("testns", "testapp")

	reconciller := &HTTPScaledObjectReconciler{
		Client:               testInfra.cl,
		Scheme:               testInfra.cl.Scheme(),
		ExternalScalerConfig: config.ExternalScaler{},
		BaseConfig:           config.Base{},
	}

	// Create required app objects for the application defined by the CRD
	err := reconciller.createOrUpdateApplicationResources(
		testInfra.ctx,
		testInfra.logger,
		testInfra.cl,
		config.Base{},
		config.ExternalScaler{},
		&testInfra.httpso,
	)
	r.NoError(err)

	// check for scaledobject, expect no error as scaledobject should get created
	_, err = getSO(
		testInfra.ctx,
		testInfra.cl,
		testInfra.httpso,
	)
	r.NoError(err)
}

func TestHttpScaledObjectControllerWhenSkipAnnotationSet(t *testing.T) {
	r := require.New(t)

	testInfra := newCommonTestInfraWithSkipScaledObjectCreation("testns", "testapp")

	reconciller := &HTTPScaledObjectReconciler{
		Client:               testInfra.cl,
		Scheme:               testInfra.cl.Scheme(),
		ExternalScalerConfig: config.ExternalScaler{},
		BaseConfig:           config.Base{},
	}

	// Create required app objects for the application defined by the CRD
	err := reconciller.createOrUpdateApplicationResources(
		testInfra.ctx,
		testInfra.logger,
		testInfra.cl,
		config.Base{},
		config.ExternalScaler{},
		&testInfra.httpso,
	)
	r.NoError(err)

	// check for scaledobject, expect error as scaledobject should not exist when skipScaledObjectCreation annotation is set
	_, err = getSO(
		testInfra.ctx,
		testInfra.cl,
		testInfra.httpso,
	)
	r.Error(err)
}

func TestHttpScaledObjectControllerWhenSkipAnnotationAddedToExistingHttpSo(t *testing.T) {
	r := require.New(t)

	testInfra := newCommonTestInfra("testns", "testapp")

	reconciller := &HTTPScaledObjectReconciler{
		Client:               testInfra.cl,
		Scheme:               testInfra.cl.Scheme(),
		ExternalScalerConfig: config.ExternalScaler{},
		BaseConfig:           config.Base{},
	}

	// Create required app objects for the application defined by the CRD
	err := reconciller.createOrUpdateApplicationResources(
		testInfra.ctx,
		testInfra.logger,
		testInfra.cl,
		config.Base{},
		config.ExternalScaler{},
		&testInfra.httpso,
	)
	r.NoError(err)

	// check for scaledobject, expect no error as scaledobject should exist when skipScaledObjectCreation annotation is not set
	_, err = getSO(
		testInfra.ctx,
		testInfra.cl,
		testInfra.httpso,
	)
	r.NoError(err)

	// add skipScaledObjectCreation annotation to HTTPScaledObject
	testInfra = newCommonTestInfraWithSkipScaledObjectCreation("testns", "testapp")

	// update required app objects for the application defined by the CRD
	err = reconciller.createOrUpdateApplicationResources(
		testInfra.ctx,
		testInfra.logger,
		testInfra.cl,
		config.Base{},
		config.ExternalScaler{},
		&testInfra.httpso,
	)
	r.NoError(err)

	// check for scaledobject, expect error as scaledobject should not exist when skipScaledObjectCreation annotation is set
	_, err = getSO(
		testInfra.ctx,
		testInfra.cl,
		testInfra.httpso,
	)
	r.Error(err)
}
