// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package awss3source

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	hookv1 "github.com/triggermesh/scoby/pkg/hook/v1"

	"github.com/triggermesh/scoby-hook-triggermesh/pkg/apis/sources/v1alpha1"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/handler"
	s3client "github.com/triggermesh/scoby-hook-triggermesh/pkg/sources/client/s3"
)

type AWSS3Handler struct {
	gvr  schema.GroupVersionResource
	kind string

	// Getter than can obtain clients for interacting with the S3 and SQS APIs
	s3Cg s3client.ClientGetter
	log  *zap.SugaredLogger
}

var _ handler.Handler = (*AWSS3Handler)(nil)

func New(s3Cg s3client.ClientGetter, log *zap.SugaredLogger) *AWSS3Handler {
	return &AWSS3Handler{
		gvr: schema.GroupVersionResource{
			Group:    "sources.triggermesh.io",
			Version:  "v1alpha1",
			Resource: "awss3sources",
		},
		kind: "AWSS3Source",

		s3Cg: s3Cg,
		log:  log,
	}
}

func (h *AWSS3Handler) GroupVersionResource() *schema.GroupVersionResource {
	return &h.gvr
}

// Kind for the managed object
func (h *AWSS3Handler) Kind() string {
	return h.kind
}

func newSubscribedCondition() *commonv1alpha1.Condition {
	return &commonv1alpha1.Condition{
		Type:   "Subscribed",
		Status: metav1.ConditionUnknown,
		Reason: "Unknown",
	}
}

func (h *AWSS3Handler) Reconcile(ctx context.Context, obj metav1.Object) *hookv1.HookResponse {
	src := obj.(*v1alpha1.AWSS3Source)

	// intialize response
	res := &hookv1.HookResponse{
		Status: &hookv1.HookStatus{
			Conditions: []commonv1alpha1.Condition{
				{
					Type:   "Subscribed",
					Status: metav1.ConditionUnknown,
					Reason: "Unknown",
				},
			},
		},
	}

	return h.reconcile(ctx, src, res)
}

func (h *AWSS3Handler) reconcile(ctx context.Context, src *v1alpha1.AWSS3Source, res *hookv1.HookResponse) *hookv1.HookResponse {
	s3Client, sqsClient, err := h.s3Cg.Get(src)
	if err != nil {
		subscribed := res.Status.Conditions.GetByType("Subscribed")
		if subscribed == nil {
			// Panic protection, this should not happen
			h.log.Error("Subscrbed condition not found")
			return res
		}
		subscribed.Status = metav1.ConditionFalse
		subscribed.Reason = "NoClient"
		subscribed.Message = "Cannot obtain AWS API clients"
		h.log.Error("Error creating AWS API clients", zap.Error(err))
		return res
	}

	queueARN, err := EnsureQueue(ctx, src, sqsClient)
	if err != nil {
		subscribed := res.Status.Conditions[0]
		subscribed.Status = metav1.ConditionFalse
		subscribed.Reason = "NoClient"
		subscribed.Message = "Cannot obtain AWS API clients"

		return fmt.Errorf("failed to reconcile SQS queue: %w", err)
	}

	return res
}

func (h *AWSS3Handler) Finalize(ctx context.Context, obj metav1.Object) *hookv1.HookResponse {
	return &hookv1.HookResponse{
		Status: &hookv1.HookStatus{
			Annotations: map[string]string{
				"io.triggermesh.hook/my-annotation": "deletion ok",
			},
		},
	}
}
