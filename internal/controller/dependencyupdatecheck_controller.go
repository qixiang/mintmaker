/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"time"

	appstudiov1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	mmv1alpha1 "github.com/konflux-ci/mintmaker/api/v1alpha1"
	component "github.com/konflux-ci/mintmaker/internal/pkg/component"
	utils "github.com/konflux-ci/mintmaker/internal/pkg/tekton"
	. "github.com/konflux-ci/mintmaker/pkg/common"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	RenovateImageEnvName    = "RENOVATE_IMAGE"
	DefaultRenovateImageUrl = "quay.io/konflux-ci/mintmaker-renovate-image:latest"
)

// DependencyUpdateCheckReconciler reconciles a DependencyUpdateCheck object
type DependencyUpdateCheckReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

func NewDependencyUpdateCheckReconciler(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder) *DependencyUpdateCheckReconciler {
	return &DependencyUpdateCheckReconciler{
		Client: client,
		Scheme: scheme,
	}
}

// createPipelineRun creates and returns a new PipelineRun
func (r *DependencyUpdateCheckReconciler) createPipelineRun(comp component.GitComponent, ctx context.Context) (*tektonv1.PipelineRun, error) {

	log := ctrllog.FromContext(ctx).WithName("DependencyUpdateCheckController")
	ctx = ctrllog.IntoContext(ctx, log)
	name := fmt.Sprintf("renovate-%d-%s-%s", comp.GetTimestamp(), RandomString(8), comp.GetName())

	// Create ConfigMap for Renovate global configuration
	renovateConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: MintMakerNamespaceName,
		},
		// TODO: replace renovate.json with Comp.GetRenovateConfig() (to be implemented)
		Data: map[string]string{
			"renovate.json": `{"$schema": "https://docs.renovatebot.com/renovate-schema.json"}`,
		},
	}

	if err := r.Client.Create(ctx, renovateConfigMap); err != nil {
		return nil, err
	}

	// Create Secret for Renovate token (the repository access token)
	renovateToken, err := comp.GetToken()
	if err != nil {
		return nil, err
	}
	renovateSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: MintMakerNamespaceName,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"renovate-token": renovateToken,
		},
	}

	if err := r.Client.Create(ctx, renovateSecret); err != nil {
		return nil, err
	}

	// Creating the pipelineRun definition
	builder := utils.NewPipelineRunBuilder(name, MintMakerNamespaceName).
		WithLabels(map[string]string{
			"mintmaker.appstudio.redhat.com/application":  comp.GetApplication(),
			"mintmaker.appstudio.redhat.com/component":    comp.GetName(),
			"mintmaker.appstudio.redhat.com/git-platform": comp.GetPlatform(), // (github, gitlab)
			"mintmaker.appstudio.redhat.com/git-host":     comp.GetHost(),     // github.com, gitlab.com, gitlab.other.com
		})

	cmItems := []corev1.KeyToPath{
		{
			Key:  "renovate.json",
			Path: "renovate.json",
		},
	}
	cmOpts := utils.NewMountOptions().WithTaskName("build").WithStepNames([]string{"renovate"})
	builder.WithConfigMap(name, "/etc/renovate/config", cmItems, cmOpts)
	secretItems := []corev1.KeyToPath{
		{
			Key:  "renovate-token",
			Path: "renovate-token",
		},
	}
	secretOpts := utils.NewMountOptions().WithTaskName("build").WithStepNames([]string{"renovate"})
	builder.WithSecret(name, "/etc/renovate/secret", secretItems, secretOpts)

	pipelineRun, err := builder.Build()
	if err != nil {
		log.Error(err, "failed to build pipeline definition")
		return nil, err
	}
	if err := r.Client.Create(ctx, pipelineRun); err != nil {
		return nil, err
	}

	// TODO: set ownerReferences for renovateConfigMap and renovateSecret
	return pipelineRun, nil
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *DependencyUpdateCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	log := ctrllog.FromContext(ctx).WithName("DependencyUpdateCheckController")
	ctx = ctrllog.IntoContext(ctx, log)

	// Ignore CRs that are not from the mintmaker namespace
	if req.Namespace != MintMakerNamespaceName {
		return ctrl.Result{}, nil
	}

	dependencyupdatecheck := &mmv1alpha1.DependencyUpdateCheck{}
	err := r.Client.Get(ctx, req.NamespacedName, dependencyupdatecheck)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
	}

	// If the DependencyUpdateCheck has been handled before, skip it
	if value, exists := dependencyupdatecheck.Annotations[MintMakerProcessedAnnotationName]; exists && value == "true" {
		log.Info(fmt.Sprintf("DependencyUpdateCheck has been processed: %v", req.NamespacedName))
		//return ctrl.Result{}, nil
	}

	// Update the DependencyUpdateCheck to add a processed annotation
	log.Info(fmt.Sprintf("new DependencyUpdateCheck found: %v", req.NamespacedName))
	if dependencyupdatecheck.Annotations == nil {
		dependencyupdatecheck.Annotations = map[string]string{}
	}
	dependencyupdatecheck.Annotations[MintMakerProcessedAnnotationName] = "true"

	err = r.Client.Update(ctx, dependencyupdatecheck)
	if err != nil {
		log.Error(err, "failed to update DependencyUpdateCheck annotations")
		return ctrl.Result{}, nil
	}

	var gatheredComponents []appstudiov1alpha1.Component
	if len(dependencyupdatecheck.Spec.Workspaces) > 0 {
		log.Info(fmt.Sprintf("Following components are specified: %v", dependencyupdatecheck.Spec.Workspaces))
		gatheredComponents, err = getFilteredComponents(dependencyupdatecheck.Spec.Workspaces, r.Client, ctx)
		if err != nil {
			log.Error(err, "gathering filtered components has failed")
			return ctrl.Result{}, err
		}
	} else {
		allComponents := &appstudiov1alpha1.ComponentList{}
		if err := r.Client.List(ctx, allComponents, &client.ListOptions{}); err != nil {
			log.Error(err, "failed to list Components")
			return ctrl.Result{}, err
		}
		gatheredComponents = allComponents.Items
	}

	log.Info(fmt.Sprintf("%d components will be processed", len(gatheredComponents)))

	// Filter out components which have mintmaker disabled
	componentList := []appstudiov1alpha1.Component{}
	for _, component := range gatheredComponents {
		if value, exists := component.Annotations[MintMakerDisabledAnnotationName]; !exists || value != "true" {
			componentList = append(componentList, component)
		}
	}

	log.Info("found components with mintmaker disabled", "components", len(gatheredComponents)-len(componentList))
	if len(componentList) == 0 {
		return ctrl.Result{}, nil
	}

	timestamp := time.Now().UTC().Unix()
	for _, appstudioComponent := range componentList {
		comp, err := component.NewGitComponent(&appstudioComponent, timestamp, r.Client, ctx)
		if err != nil {
			log.Error(err, fmt.Sprintf("failed to handle component: %s", appstudioComponent.Name))
			continue
		}
		log.Info("creating pending PipelineRun")
		pipelinerun, err := r.createPipelineRun(comp, ctx)
		if err != nil {
			log.Error(err, "failed to create PipelineRun")
		} else {
			log.Info(fmt.Sprintf("created PipelineRun %s", pipelinerun.Name))
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DependencyUpdateCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// we are monitoring the creation of DependencyUpdateCheck
	return ctrl.NewControllerManagedBy(mgr).
		For(&mmv1alpha1.DependencyUpdateCheck{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc:  func(createEvent event.CreateEvent) bool { return true },
			DeleteFunc:  func(deleteEvent event.DeleteEvent) bool { return false },
			UpdateFunc:  func(updateEvent event.UpdateEvent) bool { return false },
			GenericFunc: func(genericEvent event.GenericEvent) bool { return false },
		}).
		Complete(r)
}
