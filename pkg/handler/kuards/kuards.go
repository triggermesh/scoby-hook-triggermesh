// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package kuards

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	hookv1 "github.com/triggermesh/scoby/pkg/hook/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/triggermesh/scoby-hook-triggermesh/pkg/handler"
)

type KuardHandler struct {
	gvr  schema.GroupVersionResource
	kind string
}

var _ handler.Handler = (*KuardHandler)(nil)

func New() *KuardHandler {
	return &KuardHandler{
		gvr: schema.GroupVersionResource{
			Group:    "extensions.triggermesh.io",
			Version:  "v1",
			Resource: "kuards",
		},
		kind: "Kuard",
	}
}

func (h *KuardHandler) GroupVersionResource() *schema.GroupVersionResource {
	return &h.gvr
}

// Kind for the managed object
func (h *KuardHandler) Kind() string {
	return h.kind
}

func (h *KuardHandler) Reconcile(obj metav1.Object) *hookv1.HookResponse {
	return &hookv1.HookResponse{
		Status: &hookv1.HookStatus{
			Conditions: commonv1alpha1.Conditions{
				{
					Type:   "HookReportedStatus",
					Status: metav1.ConditionTrue,
					Reason: "HOOKREPORTSOK",
				},
			},
			Annotations: map[string]string{
				"io.triggermesh.hook/my-annotation": "annotation from hook",
			},
		},
		EnvVars: []corev1.EnvVar{
			{
				Name:  "FROM_HOOK_NAME",
				Value: obj.GetName(),
			},
			{
				Name:  "FROM_HOOK_NAMESPACE",
				Value: obj.GetNamespace(),
			},
		},
	}
}

func (h *KuardHandler) Finalize(obj metav1.Object) *hookv1.HookResponse {
	return &hookv1.HookResponse{
		Status: &hookv1.HookStatus{
			Annotations: map[string]string{
				"io.triggermesh.hook/my-annotation": "deletion ok",
			},
		},
	}
}
