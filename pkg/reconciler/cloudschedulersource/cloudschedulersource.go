/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cloudschedulersource

import (
	"context"
	"fmt"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/logging/logkey"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	extv1beta1informers "k8s.io/client-go/informers/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	extlisters "k8s.io/client-go/listers/extensions/v1beta1"

	"cloud.google.com/go/scheduler/apiv1beta1"
	"github.com/vaikas-google/csr/pkg/apis/cloudschedulersource/v1alpha1"
	clientset "github.com/vaikas-google/csr/pkg/client/clientset/versioned"
	cloudschedulersourcescheme "github.com/vaikas-google/csr/pkg/client/clientset/versioned/scheme"
	informers "github.com/vaikas-google/csr/pkg/client/informers/externalversions/cloudschedulersource/v1alpha1"
	listers "github.com/vaikas-google/csr/pkg/client/listers/cloudschedulersource/v1alpha1"
	"github.com/vaikas-google/csr/pkg/reconciler/cloudschedulersource/resources"
	schedulerpb "google.golang.org/genproto/googleapis/cloud/scheduler/v1beta1"
)

const controllerAgentName = "cloudschedulersource-controller"

// Reconciler is the controller implementation for Cloudschedulersource resources
type Reconciler struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// cloudschedulersourceclientset is a clientset for our own API group
	cloudschedulersourceclientset clientset.Interface

	daemonsetsLister            extlisters.DaemonSetLister
	cloudschedulersourcesLister listers.CloudSchedulerSourceLister

	sleeperImage string

	// Sugared logger is easier to use but is not as performant as the
	// raw logger. In performance critical paths, call logger.Desugar()
	// and use the returned raw logger instead. In addition to the
	// performance benefits, raw logger also preserves type-safety at
	// the expense of slightly greater verbosity.
	Logger *zap.SugaredLogger
}

// Check that we implement the controller.Reconciler interface.
var _ controller.Reconciler = (*Reconciler)(nil)

func init() {
	// Add cloudschedulersource-controller types to the default Kubernetes Scheme so Events can be
	// logged for cloudschedulersource-controller types.
	cloudschedulersourcescheme.AddToScheme(scheme.Scheme)
}

// NewController returns a new cloudschedulersource controller
func NewController(
	logger *zap.SugaredLogger,
	kubeclientset kubernetes.Interface,
	cloudschedulersourceclientset clientset.Interface,
	daemonsetInformer extv1beta1informers.DaemonSetInformer,
	cloudschedulersourceInformer informers.CloudSchedulerSourceInformer,
	sleeperImage string,
) *controller.Impl {

	// Enrich the logs with controller name
	logger = logger.Named(controllerAgentName).With(zap.String(logkey.ControllerType, controllerAgentName))

	r := &Reconciler{
		kubeclientset:                 kubeclientset,
		cloudschedulersourceclientset: cloudschedulersourceclientset,
		daemonsetsLister:              daemonsetInformer.Lister(),
		cloudschedulersourcesLister:   cloudschedulersourceInformer.Lister(),
		sleeperImage:                  sleeperImage,
		Logger:                        logger,
	}
	impl := controller.NewImpl(r, logger, "CloudSchedulerSources")

	logger.Info("Setting up event handlers")
	// Set up an event handler for when CloudSchedulerSource resources change
	cloudschedulersourceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    impl.Enqueue,
		UpdateFunc: controller.PassNew(impl.Enqueue),
	})

	return impl
}

// Reconcile implements controller.Reconciler
func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the CloudSchedulerSource resource with this namespace/name
	csr, err := c.cloudschedulersourcesLister.CloudSchedulerSources(namespace).Get(name)
	if errors.IsNotFound(err) {
		// The CloudSchedulerSource resource may no longer exist, in which case we stop processing.
		runtime.HandleError(fmt.Errorf("cloudschedulersource '%s' in work queue no longer exists", key))
		return nil
	} else if err != nil {
		return err
	}

	if err := c.reconcileCloudSchedulerSource(ctx, csr); err != nil {
		return err
	}

	return nil
}

func (c *Reconciler) reconcileCloudSchedulerSource(ctx context.Context, csr *v1alpha1.CloudSchedulerSource) error {
	return nil
}

func (c *Reconciler) createJob(spec *v1alpha1.CloudSchedulerSourceSpec) error {
	ctx := context.Background()
	csc, err := scheduler.NewCloudSchedulerClient(ctx)
	if err != nil {
		return err
	}

	req := &schedulerpb.CreateJobRequest{
		Parent: "/projects/quantum-reducer-434/locations/us-central1",
		Job: &schedulerpb.Job{
			Name: "vaikastest",
			Target: &schedulerpb.Job_HttpTarget{
				HttpTarget: &schedulerpb.HttpTarget{
					Uri:        "http://message-dumper.default.aikas.org/",
					HttpMethod: schedulerpb.HttpMethod_POST,
				},
			},
		},
	}
	resp, err := csc.CreateJob(ctx, req)
	if err != nil {
		return err
	}
	// TODO: Use resp.
	c.Logger.Infof("Created job %+v", resp)
	return nil
}
