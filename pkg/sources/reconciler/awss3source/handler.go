// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package awss3source

import (
	"context"

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

// func newSubscribedCondition() *commonv1alpha1.Condition {
// 	return &commonv1alpha1.Condition{
// 		Type:   "Subscribed",
// 		Status: metav1.ConditionUnknown,
// 		Reason: "Unknown",
// 	}
// }

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

	h.reconcile(ctx, src, res)

	return res
}

func (h *AWSS3Handler) reconcile(ctx context.Context, src *v1alpha1.AWSS3Source, res *hookv1.HookResponse) {
	s3Client, sqsClient, err := h.s3Cg.Get(src)
	if err != nil {
		subscribed := res.Status.Conditions.GetByType("Subscribed")
		if subscribed == nil {
			// Panic protection, this should not happen
			h.log.Error("Subscribed condition not found")
			return
		}
		subscribed.Status = metav1.ConditionFalse
		subscribed.Reason = "NoClient"
		subscribed.Message = "Cannot obtain AWS API clients"
		h.log.Error("Error creating AWS API clients", zap.Error(err))
		return
	}

	queueARN, err := EnsureQueue(ctx, src, sqsClient)
	if err != nil {
		subscribed := res.Status.Conditions[0]
		subscribed.Status = metav1.ConditionFalse
		subscribed.Reason = "ReconcileQueue"
		subscribed.Message = "Failed to reconcile SQS queue"
		h.log.Error("Failed to reconcile SQS queue", zap.Error(err))
		return
	}

	err = EnsureNotificationsEnabled(ctx, src, s3Client, queueARN)
	if err != nil {
		subscribed := res.Status.Conditions[0]
		subscribed.Status = metav1.ConditionFalse
		subscribed.Reason = "ConfigureNotifications"
		subscribed.Message = "Cannot configure SQS notifications"
		h.log.Error("Failed to configure SQS queue notifications", zap.Error(err))
		return
	}

	subscribed := res.Status.Conditions[0]
	subscribed.Status = metav1.ConditionTrue
	subscribed.Reason = ""

	return
}

func (h *AWSS3Handler) Finalize(ctx context.Context, obj metav1.Object) *hookv1.HookResponse {
	src := obj.(*v1alpha1.AWSS3Source)
	res := &hookv1.HookResponse{
		Status: &hookv1.HookStatus{
			Conditions: []commonv1alpha1.Condition{
				{
					Type: "Subscribed",
					// True being set as the default value means that we are
					// ok removing the component.
					Status: metav1.ConditionTrue,
					Reason: "",
				},
			},
		},
	}

	h.finalize(ctx, src, res)

	return res
}

func (h *AWSS3Handler) finalize(ctx context.Context, src *v1alpha1.AWSS3Source, res *hookv1.HookResponse) {
	s3Client, sqsClient, err := h.s3Cg.Get(src)
	switch {
	case isNotFound(err):
		// the finalizer is unlikely to recover from a missing Secret,
		// so we simply record a warning event and return
		h.log.Error("Secret missing while finalizing event source. Ignoring", zap.Error(err))
		return
	case err != nil:
		h.log.Error("Error creating AWS API clients", zap.Error(err))
		subscribed := res.Status.Conditions.GetByType("Subscribed")
		if subscribed == nil {
			// Panic protection, this should not happen
			h.log.Error("Subscribed condition not found", zap.Error(err))
			return
		}

		subscribed.Status = metav1.ConditionFalse
		subscribed.Reason = "NoClient"
		subscribed.Message = "Cannot obtain AWS API clients"
		return
	}

	if err := EnsureNoQueue(ctx, src, sqsClient); err != nil {
		h.log.Error("Failed to finalize SQS queue", zap.Error(err))
	}

	// The finalizer blocks the deletion of the source object until
	// ensureNotificationsDisabled succeeds to ensure that we don't leave
	// any dangling event notification configurations behind us.
	if err := EnsureNotificationsDisabled(ctx, src, s3Client); err != nil {
		h.log.Error("Failed to disable S3 notifications", zap.Error(err))
	}
}

// sourceID returns an ID that identifies the given source instance in AWS
// resources or resources tags.
func sourceID(src metav1.Object) string {
	return "io.triggermesh.awss3sources." + src.GetNamespace() + "." + src.GetName()
}
