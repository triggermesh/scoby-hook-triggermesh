// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package awss3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	hookv1 "github.com/triggermesh/scoby/pkg/hook/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/triggermesh/scoby-hook-triggermesh/pkg/handler"
	s3client "github.com/triggermesh/scoby-hook-triggermesh/pkg/sources/client/s3"
)

type AWSS3Handler struct {
	gvr  schema.GroupVersionResource
	kind string

	// Getter than can obtain clients for interacting with the S3 and SQS APIs
	s3Cg s3client.ClientGetter
}

var _ handler.Handler = (*AWSS3Handler)(nil)

func New() *AWSS3Handler {
	return &AWSS3Handler{
		gvr: schema.GroupVersionResource{
			Group:    "sources.triggermesh.io",
			Version:  "v1alpha1",
			Resource: "awss3sources",
		},
		kind: "AWSS3Source",
	}
}

func (h *AWSS3Handler) GroupVersionResource() *schema.GroupVersionResource {
	return &h.gvr
}

// Kind for the managed object
func (h *AWSS3Handler) Kind() string {
	return h.kind
}

func (h *AWSS3Handler) Reconcile(obj metav1.Object) *hookv1.HookResponse {
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

func (h *AWSS3Handler) Finalize(obj metav1.Object) *hookv1.HookResponse {
	return &hookv1.HookResponse{
		Status: &hookv1.HookStatus{
			Annotations: map[string]string{
				"io.triggermesh.hook/my-annotation": "deletion ok",
			},
		},
	}
}
