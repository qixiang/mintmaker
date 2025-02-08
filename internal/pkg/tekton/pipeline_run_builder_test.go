// Copyright 2024 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tekton

import (
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PipelineRun builder", func() {

	When("NewPipelineRunBuilder method is called", func() {
		var (
			name      = "testName"
			namespace = "testNamespace"
			builder   *PipelineRunBuilder
		)

		BeforeEach(func() {
			builder = NewPipelineRunBuilder(name, namespace)
		})

		It("should return a new PipelineRunBuilder instance", func() {
			Expect(builder).To(Not(BeNil()))
		})

		It("should set the correct Name in the returned PipelineRunBuilder instance", func() {
			Expect(builder.pipelineRun.ObjectMeta.Name).To(Equal(name))
		})

		It("should set the correct Namespace in the returned PipelineRunBuilder instance", func() {
			Expect(builder.pipelineRun.ObjectMeta.Namespace).To(Equal(namespace))
		})

		It("should initialize an empty PipelineRunSpec", func() {
			Expect(builder.pipelineRun.Spec).To(Equal(tektonv1.PipelineRunSpec{}))
		})
	})

	When("Build method is called", func() {
		It("should return the constructed PipelineRun if there are no errors", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			pr, err := builder.Build()
			Expect(pr).To(Not(BeNil()))
			Expect(err).To(BeNil())
		})

		It("should return the accumulated errors", func() {
			builder := &PipelineRunBuilder{
				err: multierror.Append(nil, fmt.Errorf("dummy error 1"), fmt.Errorf("dummy error 2")),
			}
			_, err := builder.Build()
			Expect(err).To(Not(BeNil()))
			Expect(err.Error()).To(ContainSubstring("dummy error 1"))
			Expect(err.Error()).To(ContainSubstring("dummy error 2"))
		})
	})

	When("WithAnnotations method is called", func() {
		var (
			builder *PipelineRunBuilder
		)

		BeforeEach(func() {
			builder = NewPipelineRunBuilder("testPrefix", "testNamespace")
		})

		It("should add annotations when none previously existed", func() {
			builder.WithAnnotations(map[string]string{
				"annotation1": "value1",
				"annotation2": "value2",
			})
			Expect(builder.pipelineRun.ObjectMeta.Annotations).To(HaveKeyWithValue("annotation1", "value1"))
			Expect(builder.pipelineRun.ObjectMeta.Annotations).To(HaveKeyWithValue("annotation2", "value2"))
		})

		It("should update existing annotations and add new ones", func() {
			builder.pipelineRun.ObjectMeta.Annotations = map[string]string{
				"annotation1": "oldValue1",
				"annotation3": "value3",
			}
			builder.WithAnnotations(map[string]string{
				"annotation1": "newValue1",
				"annotation2": "value2",
			})
			Expect(builder.pipelineRun.ObjectMeta.Annotations).To(HaveKeyWithValue("annotation1", "newValue1"))
			Expect(builder.pipelineRun.ObjectMeta.Annotations).To(HaveKeyWithValue("annotation2", "value2"))
			Expect(builder.pipelineRun.ObjectMeta.Annotations).To(HaveKeyWithValue("annotation3", "value3"))
		})
	})

	When("WithFinalizer method is called", func() {
		var (
			builder *PipelineRunBuilder
		)

		BeforeEach(func() {
			builder = NewPipelineRunBuilder("testPrefix", "testNamespace")
		})

		It("should add a finalizer when none previously existed", func() {
			builder.WithFinalizer("finalizer1")
			Expect(builder.pipelineRun.ObjectMeta.Finalizers).To(ContainElement("finalizer1"))
		})

		It("should append a new finalizer to the existing finalizers", func() {
			builder.pipelineRun.ObjectMeta.Finalizers = []string{"existingFinalizer"}
			builder.WithFinalizer("finalizer2")
			Expect(builder.pipelineRun.ObjectMeta.Finalizers).To(ContainElements("existingFinalizer", "finalizer2"))
		})
	})

	When("WithLabels method is called", func() {
		var (
			builder *PipelineRunBuilder
		)

		BeforeEach(func() {
			builder = NewPipelineRunBuilder("testPrefix", "testNamespace")
		})

		It("should add labels when none previously existed", func() {
			builder.WithLabels(map[string]string{
				"label1": "value1",
				"label2": "value2",
			})
			Expect(builder.pipelineRun.ObjectMeta.Labels).To(HaveKeyWithValue("label1", "value1"))
			Expect(builder.pipelineRun.ObjectMeta.Labels).To(HaveKeyWithValue("label2", "value2"))
		})

		It("should update existing labels and add new ones", func() {
			builder.pipelineRun.ObjectMeta.Labels = map[string]string{
				"label1": "oldValue1",
				"label3": "value3",
			}
			builder.WithLabels(map[string]string{
				"label1": "newValue1",
				"label2": "value2",
			})
			Expect(builder.pipelineRun.ObjectMeta.Labels).To(HaveKeyWithValue("label1", "newValue1"))
			Expect(builder.pipelineRun.ObjectMeta.Labels).To(HaveKeyWithValue("label2", "value2"))
			Expect(builder.pipelineRun.ObjectMeta.Labels).To(HaveKeyWithValue("label3", "value3"))
		})
	})

	When("WithObjectReferences method is called", func() {
		It("should add parameters based on the provided client.Objects", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			configMap1 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configName1",
					Namespace: "configNamespace1",
				},
			}
			configMap1.Kind = "ConfigMap"
			configMap2 := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "configName2",
					Namespace: "configNamespace2",
				},
			}
			configMap2.Kind = "ConfigMap"

			builder.WithObjectReferences(configMap1, configMap2)

			Expect(builder.pipelineRun.Spec.Params).To(ContainElement(tektonv1.Param{
				Name:  "configMap",
				Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: "configNamespace1/configName1"},
			}))
			Expect(builder.pipelineRun.Spec.Params).To(ContainElement(tektonv1.Param{
				Name:  "configMap",
				Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: "configNamespace2/configName2"},
			}))
		})
	})

	When("WithObjectSpecsAsJson method is called", func() {
		It("should add parameters with JSON representation of the object's Spec", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			pod1 := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container1",
							Image: "image1",
						},
					},
				},
			}
			pod1.Kind = "Pod"
			pod2 := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container2",
							Image: "image2",
						},
					},
				},
			}
			pod2.Kind = "Pod"

			builder.WithObjectSpecsAsJson(pod1, pod2)

			Expect(builder.pipelineRun.Spec.Params).To(ContainElement(tektonv1.Param{
				Name: "pod",
				Value: tektonv1.ParamValue{
					Type:      tektonv1.ParamTypeString,
					StringVal: `{"containers":[{"name":"container1","image":"image1","resources":{}}]}`,
				},
			}))
			Expect(builder.pipelineRun.Spec.Params).To(ContainElement(tektonv1.Param{
				Name: "pod",
				Value: tektonv1.ParamValue{
					Type:      tektonv1.ParamTypeString,
					StringVal: `{"containers":[{"name":"container2","image":"image2","resources":{}}]}`,
				},
			}))
		})
	})

	When("WithParams method is called", func() {
		It("should append the provided parameters to the PipelineRun's spec", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")

			param1 := tektonv1.Param{
				Name:  "param1",
				Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: "value1"},
			}
			param2 := tektonv1.Param{
				Name:  "param2",
				Value: tektonv1.ParamValue{Type: tektonv1.ParamTypeString, StringVal: "value2"},
			}

			builder.WithParams(param1, param2)

			Expect(builder.pipelineRun.Spec.Params).To(ContainElements(param1, param2))
		})
	})

	When("WithServiceAccount method is called", func() {
		It("should set the ServiceAccountName for the PipelineRun's TaskRunTemplate", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			serviceAccount := "sampleServiceAccount"
			builder.WithServiceAccount(serviceAccount)
			Expect(builder.pipelineRun.Spec.TaskRunTemplate.ServiceAccountName).To(Equal(serviceAccount))
		})
	})

	When("WithTimeouts method is called", func() {
		It("should set the timeouts for the PipelineRun", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			timeouts := &tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
				Tasks:    &metav1.Duration{Duration: 1 * time.Hour},
				Finally:  &metav1.Duration{Duration: 1 * time.Hour},
			}
			builder.WithTimeouts(timeouts, nil)
			Expect(builder.pipelineRun.Spec.Timeouts).To(Equal(timeouts))
		})

		It("should use the default timeouts if the given timeouts are empty", func() {
			builder := NewPipelineRunBuilder("testPrefix", "testNamespace")
			defaultTimeouts := &tektonv1.TimeoutFields{
				Pipeline: &metav1.Duration{Duration: 1 * time.Hour},
				Tasks:    &metav1.Duration{Duration: 1 * time.Hour},
				Finally:  &metav1.Duration{Duration: 1 * time.Hour},
			}
			builder.WithTimeouts(nil, defaultTimeouts)
			Expect(builder.pipelineRun.Spec.Timeouts).To(Equal(defaultTimeouts))
		})
	})
})
