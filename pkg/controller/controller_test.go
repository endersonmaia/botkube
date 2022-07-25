package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/kubeshop/botkube/pkg/config"
	"github.com/kubeshop/botkube/pkg/utils"
)

// TODO: Refactor these tests as a part of https://github.com/kubeshop/botkube/issues/589
//  These tests were moved from old E2E package with fake K8s and Slack API
//  (deleted in https://github.com/kubeshop/botkube/pull/627) and adjusted to become unit tests.

// ErrorEvent stores specs for throwing an error in case of anomalies
type ErrorEvent struct {
	GVR       schema.GroupVersionResource
	Kind      string
	Namespace string
	Specs     runtime.Object
}

// UpdateObjects stores specs and patch for updating a k8s fake object and expected Slack response
type UpdateObjects struct {
	GVR       schema.GroupVersionResource
	Kind      string
	Namespace string
	Specs     runtime.Object
}

// DeleteObjects stores specs for deleting a k8s fake object
type DeleteObjects struct {
	GVR       schema.GroupVersionResource
	Kind      string
	Namespace string
	Specs     runtime.Object
}

func TestController_ShouldSendEvent_SkipError(t *testing.T) {
	observedEventKindsMap := map[EventKind]bool{
		{Resource: "v1/pods", Namespace: "dummy", EventType: "error"}: true,
	}

	tests := map[string]ErrorEvent{
		"skip error event for resources not configured": {
			// error event not allowed for Pod resources so event should be skipped
			GVR:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      "Pod",
			Namespace: "test",
			Specs:     &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod"}, Spec: v1.PodSpec{Containers: []v1.Container{{Name: "test-pod-container", Image: "tomcat:9.0.34"}}}},
		},
		"skip error event for namespace not configured": {
			// error event not allowed for Pod in test namespace so event should be skipped
			GVR:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      "Pod",
			Namespace: "test",
			Specs:     &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod"}, Spec: v1.PodSpec{Containers: []v1.Container{{Name: "test-pod-container", Image: "tomcat:9.0.34"}}}},
		},
		"skip error event for resources not added in test_config": {
			// error event should not be allowed for service resource so event should be skipped
			GVR:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
			Kind:      "Service",
			Namespace: "test",
			Specs:     &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test-service-error"}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resource := utils.GVRToString(test.GVR)

			c := Controller{
				observedEventKindsMap: observedEventKindsMap,
			}

			isAllowed := c.shouldSendEvent(test.Namespace, resource, config.ErrorEvent)
			assert.Equal(t, false, isAllowed)
		})
	}
}

func TestController_ShouldSendEvent_SkipUpdate(t *testing.T) {
	observedEventKindsMap := map[EventKind]bool{
		{Resource: "v1/pods", Namespace: "dummy", EventType: "delete"}: true,
	}

	// test scenarios
	tests := map[string]UpdateObjects{
		"skip update event for namespaces not configured": {
			// update operation not allowed for Pod in test namespace so event should be skipped
			GVR:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      "Pod",
			Namespace: "test",
			Specs:     &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod-update"}, Spec: v1.PodSpec{Containers: []v1.Container{{Name: "test-pod-container", Image: "tomcat:9.0.34"}}}},
		},
		"skip update event for resources not added": {
			// update operation not allowed for namespaces in test_config so event should be skipped
			GVR:   schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"},
			Kind:  "Namespace",
			Specs: &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "abc"}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resource := utils.GVRToString(test.GVR)

			c := Controller{
				observedEventKindsMap: observedEventKindsMap,
			}

			isAllowed := c.shouldSendEvent(test.Namespace, resource, config.ErrorEvent)
			assert.Equal(t, false, isAllowed)
		})
	}
}

func TestController_ShouldSendEvent_SkipDelete(t *testing.T) {
	observedEventKindsMap := map[EventKind]bool{
		{Resource: "v1/pods", Namespace: "dummy", EventType: "delete"}: true,
	}

	// test scenarios
	tests := map[string]DeleteObjects{
		"skip delete event for resources not configured": {
			// delete operation not allowed for Pod resources so event should be skipped
			GVR:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      "Pod",
			Namespace: "test",
			Specs:     &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod-delete"}, Spec: v1.PodSpec{Containers: []v1.Container{{Name: "test-pod-container", Image: "tomcat:9.0.34"}}}},
		},
		"skip delete event for namespace not configured": {
			// delete operation not allowed for Pod in test namespace so event should be skipped
			GVR:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Kind:      "Pod",
			Namespace: "test",
			Specs:     &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod-delete"}, Spec: v1.PodSpec{Containers: []v1.Container{{Name: "test-pod-container", Image: "tomcat:9.0.34"}}}},
		},
		"skip delete event for resources not added in test_config": {
			// delete operation not allowed for Pod resources so event should be skipped
			GVR:       schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
			Kind:      "Service",
			Namespace: "test",
			Specs:     &v1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test-service-delete"}},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resource := utils.GVRToString(test.GVR)

			c := Controller{
				observedEventKindsMap: observedEventKindsMap,
			}

			isAllowed := c.shouldSendEvent(test.Namespace, resource, config.ErrorEvent)
			assert.Equal(t, false, isAllowed)
		})
	}
}

func TestController_strToGVR(t *testing.T) {
	// test scenarios
	tests := []struct {
		Name               string
		Input              string
		Expected           schema.GroupVersionResource
		ExpectedErrMessage string
	}{
		{
			Name:  "Without group",
			Input: "v1/persistentvolumes",
			Expected: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "persistentvolumes",
			},
		},
		{
			Name:  "With group",
			Input: "apps/v1/daemonsets",
			Expected: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "daemonsets",
			},
		},
		{
			Name:  "With more complex group",
			Input: "rbac.authorization.k8s.io/v1/clusterroles",
			Expected: schema.GroupVersionResource{
				Group:    "rbac.authorization.k8s.io",
				Version:  "v1",
				Resource: "clusterroles",
			},
		},
		{
			Name:               "Error",
			Input:              "foo/bar/baz/qux",
			ExpectedErrMessage: "invalid string: expected 2 or 3 parts when split by \"/\"",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.Name, func(t *testing.T) {
			c := Controller{}

			res, err := c.strToGVR(testCase.Input)

			if testCase.ExpectedErrMessage != "" {
				require.Error(t, err)
				assert.EqualError(t, err, testCase.ExpectedErrMessage)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.Expected, res)
		})
	}
}
