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
package tekton

import (
	"encoding/json"
	"fmt"
	"reflect"
	"unicode"

	"github.com/hashicorp/go-multierror"
	libhandler "github.com/operator-framework/operator-lib/handler"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type PipelineRunBuilder struct {
	err         *multierror.Error
	pipelineRun *tektonv1.PipelineRun
}

// NewPipelineRunBuilder initializes a new PipelineRunBuilder with the given name prefix and namespace.
// It sets the name of the PipelineRun to be generated with the provided prefix and sets its namespace.
func NewPipelineRunBuilder(name, namespace string) *PipelineRunBuilder {
	return &PipelineRunBuilder{
		pipelineRun: &tektonv1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: tektonv1.PipelineRunSpec{
				Status: tektonv1.PipelineRunSpecStatusPending,
				PipelineSpec: &tektonv1.PipelineSpec{
					Tasks: []tektonv1.PipelineTask{
						{
							Name: "build",
							TaskSpec: &tektonv1.EmbeddedTask{
								TaskSpec: tektonv1.TaskSpec{
									Steps: []tektonv1.Step{
										{
											Name:  "renovate",
											Image: "TODO: changeme",
											Script: `
	        	                            echo "Running Renovate"
	        	                            sleep 10
	        	                        `,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Build returns the constructed PipelineRun and any accumulated error.
func (b *PipelineRunBuilder) Build() (*tektonv1.PipelineRun, error) {
	return b.pipelineRun, b.err.ErrorOrNil()
}

// WithAnnotations appends or updates annotations to the PipelineRun's metadata.
// If the PipelineRun does not have existing annotations, it initializes them before adding.
func (b *PipelineRunBuilder) WithAnnotations(annotations map[string]string) *PipelineRunBuilder {
	if b.pipelineRun.ObjectMeta.Annotations == nil {
		b.pipelineRun.ObjectMeta.Annotations = make(map[string]string)
	}
	for key, value := range annotations {
		b.pipelineRun.ObjectMeta.Annotations[key] = value
	}
	return b
}

// WithFinalizer adds the given finalizer to the PipelineRun's metadata.
func (b *PipelineRunBuilder) WithFinalizer(finalizer string) *PipelineRunBuilder {
	controllerutil.AddFinalizer(b.pipelineRun, finalizer)
	return b
}

// WithLabels appends or updates labels to the PipelineRun's metadata.
// If the PipelineRun does not have existing labels, it initializes them before adding.
func (b *PipelineRunBuilder) WithLabels(labels map[string]string) *PipelineRunBuilder {
	if b.pipelineRun.ObjectMeta.Labels == nil {
		b.pipelineRun.ObjectMeta.Labels = make(map[string]string)
	}
	for key, value := range labels {
		b.pipelineRun.ObjectMeta.Labels[key] = value
	}
	return b
}

// WithObjectReferences constructs tektonv1.Param entries for each of the provided client.Objects.
// Each param name is derived from the object's Kind (with the first letter made lowercase) and
// the value is a combination of the object's Namespace and Name.
func (b *PipelineRunBuilder) WithObjectReferences(objects ...client.Object) *PipelineRunBuilder {
	for _, obj := range objects {
		name := []rune(obj.GetObjectKind().GroupVersionKind().Kind)
		name[0] = unicode.ToLower(name[0])
		b.WithParams(tektonv1.Param{
			Name: string(name),
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: obj.GetNamespace() + "/" + obj.GetName(),
			},
		})
	}
	return b
}

// WithObjectSpecsAsJson constructs tektonv1.Param entries for the Spec field of each of the provided client.Objects.
// Each param name is derived from the object's Kind (with the first letter made lowercase).
// The value for each param is the JSON representation of the object's Spec.
// If an error occurs during extraction or serialization, it's accumulated in the builder's err field using multierror.
func (b *PipelineRunBuilder) WithObjectSpecsAsJson(objects ...client.Object) *PipelineRunBuilder {
	for _, obj := range objects {
		name := []rune(obj.GetObjectKind().GroupVersionKind().Kind)
		name[0] = unicode.ToLower(name[0])
		value := reflect.ValueOf(obj).Elem().FieldByName("Spec")
		if !value.IsValid() {
			b.err = multierror.Append(b.err, fmt.Errorf("failed to extract spec for object: %s", string(name)))
			continue
		}
		jsonData, err := json.Marshal(value.Interface())
		if err != nil {
			b.err = multierror.Append(b.err, fmt.Errorf("failed to serialize spec of object %s to JSON: %v", string(name), err))
			continue
		}
		b.WithParams(tektonv1.Param{
			Name: string(name),
			Value: tektonv1.ParamValue{
				Type:      tektonv1.ParamTypeString,
				StringVal: string(jsonData),
			},
		})
	}
	return b
}

// WithOwner sets the given client.Object as the owner of the PipelineRun.
// It also adds the ReleaseFinalizer to the PipelineRun.
func (b *PipelineRunBuilder) WithOwner(object client.Object) *PipelineRunBuilder {
	if err := libhandler.SetOwnerAnnotations(object, b.pipelineRun); err != nil {
		b.err = multierror.Append(b.err, fmt.Errorf("failed to set owner annotations: %v", err))
		return b
	}
	return b
}

// WithParams appends the provided params to the PipelineRun's spec.
func (b *PipelineRunBuilder) WithParams(params ...tektonv1.Param) *PipelineRunBuilder {
	if b.pipelineRun.Spec.Params == nil {
		b.pipelineRun.Spec.Params = make([]tektonv1.Param, 0)
	}
	b.pipelineRun.Spec.Params = append(b.pipelineRun.Spec.Params, params...)
	return b
}

// WithServiceAccount sets the ServiceAccountName for the PipelineRun's TaskRunTemplate.
func (b *PipelineRunBuilder) WithServiceAccount(serviceAccount string) *PipelineRunBuilder {
	b.pipelineRun.Spec.TaskRunTemplate.ServiceAccountName = serviceAccount
	return b
}

// WithTimeouts sets the Timeouts for the PipelineRun.
func (b *PipelineRunBuilder) WithTimeouts(timeouts, defaultTimeouts *tektonv1.TimeoutFields) *PipelineRunBuilder {
	if timeouts == nil || *timeouts == (tektonv1.TimeoutFields{}) {
		b.pipelineRun.Spec.Timeouts = defaultTimeouts
	} else {
		b.pipelineRun.Spec.Timeouts = timeouts
	}
	return b
}
