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
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	appstudiov1alpha1 "github.com/konflux-ci/application-api/api/v1alpha1"
	mmv1alpha1 "github.com/konflux-ci/mintmaker/api/v1alpha1"
	. "github.com/konflux-ci/mintmaker/pkg/common"
	"github.com/konflux-ci/mintmaker/pkg/git"
	"github.com/konflux-ci/mintmaker/pkg/k8s"
	"github.com/konflux-ci/mintmaker/pkg/renovate"
)

// DependencyUpdateCheckReconciler reconciles a DependencyUpdateCheck object
type DependencyUpdateCheckReconciler struct {
	client         client.Client
	taskProviders  []renovate.TaskProvider
	eventRecorder  record.EventRecorder
	jobCoordinator *renovate.JobCoordinator
}

func NewDependencyUpdateCheckReconciler(client client.Client, scheme *runtime.Scheme, eventRecorder record.EventRecorder) *DependencyUpdateCheckReconciler {
	return &DependencyUpdateCheckReconciler{
		client: client,
		taskProviders: []renovate.TaskProvider{
			renovate.NewGithubAppRenovaterTaskProvider(k8s.NewGithubAppConfigReader(client, scheme, eventRecorder)),
			renovate.NewBasicAuthTaskProvider(k8s.NewGitCredentialProvider(client)),
		},
		eventRecorder:  eventRecorder,
		jobCoordinator: renovate.NewJobCoordinator(client, scheme),
	}
}

//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=dependencyupdatechecks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=dependencyupdatechecks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appstudio.redhat.com,resources=dependencyupdatechecks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *DependencyUpdateCheckReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithName("DependencyUpdateCheckController")
	ctx = ctrllog.IntoContext(ctx, log)

	// Ignore CRs that are not from the mintmaker namespace
	if req.Namespace != MintMakerNamespaceName {
		return ctrl.Result{}, nil
	}

	dependencyupdatecheck := &mmv1alpha1.DependencyUpdateCheck{}
	err := r.client.Get(ctx, req.NamespacedName, dependencyupdatecheck)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
	}

	// If the DependencyUpdateCheck has been handled before, skip it
	if _, processed := dependencyupdatecheck.Annotations[MintMakerProcessedAnnotationName]; processed {
		log.Info(fmt.Sprintf("DependencyUpdateCheck has been processed: %v", req.NamespacedName))
		return ctrl.Result{}, nil
	}

	// Update the DependencyUpdateCheck to add a processed annotation
	log.Info(fmt.Sprintf("new DependencyUpdateCheck found: %v", req.NamespacedName))
	if dependencyupdatecheck.Annotations == nil {
		dependencyupdatecheck.Annotations = map[string]string{}
	}
	dependencyupdatecheck.Annotations[MintMakerProcessedAnnotationName] = "true"

	err = r.client.Update(ctx, dependencyupdatecheck)
	if err != nil {
		log.Error(err, "failed to update DependencyUpdateCheck annotations")
		return ctrl.Result{}, nil
	}

	// Get Components
	allComponents := &appstudiov1alpha1.ComponentList{}
	if err := r.client.List(ctx, allComponents, &client.ListOptions{}); err != nil {
		log.Error(err, "failed to list Components")
		return ctrl.Result{}, err
	}

	log.Info("found components", "components", len(allComponents.Items))

	// Filter out components which have mintmaker disabled
	componentList := []appstudiov1alpha1.Component{}
	for _, component := range allComponents.Items {
		if value, exists := component.Annotations[MintMakerDisabledAnnotationName]; !exists || value != "true" {
			componentList = append(componentList, component)
		}
	}

	log.Info("found components with mintmaker disabled", "components", len(allComponents.Items)-len(componentList))
	if len(componentList) == 0 {
		return ctrl.Result{}, nil
	}

	var scmComponents []*git.ScmComponent
	for _, component := range componentList {
		gitProvider, err := getGitProvider(component)
		if err != nil {
			// component misconfiguration shouldn't prevent other components from being updated
			// deepcopy the component to avoid implicit memory aliasing in for loop
			r.eventRecorder.Event(component.DeepCopy(), "Warning", "ErrorComponentProviderInfo", err.Error())
			continue
		}

		scmComponent, err := git.NewScmComponent(gitProvider, component.Spec.Source.GitSource.URL, component.Spec.Source.GitSource.Revision, component.Name, component.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		scmComponents = append(scmComponents, scmComponent)
	}
	var tasks []*renovate.Task
	for _, taskProvider := range r.taskProviders {
		newTasks := taskProvider.GetNewTasks(ctx, scmComponents)
		log.Info("found new tasks", "tasks", len(newTasks), "provider", reflect.TypeOf(taskProvider).String())
		if len(newTasks) > 0 {
			tasks = append(tasks, newTasks...)
		}
	}

	if len(tasks) == 0 {
		return ctrl.Result{}, nil
	}

	log.Info("executing renovate tasks", "tasks", len(tasks))
	err = r.jobCoordinator.ExecuteWithLimits(ctx, tasks)
	if err != nil {
		log.Error(err, "failed to create a job")
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DependencyUpdateCheckReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mmv1alpha1.DependencyUpdateCheck{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return true
			},
			DeleteFunc: func(event.DeleteEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return false
			},
			GenericFunc: func(event.GenericEvent) bool {
				return false
			},
		})).
		Complete(r)
}
