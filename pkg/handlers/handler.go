package handlers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	hookv1 "github.com/triggermesh/scoby/pkg/hook/v1"
)

type HandlerD struct {
	GVR  schema.GroupVersionResource
	Kind string
}

// Handler exposes methods for hook handler registration and reconciling.
type Handler interface {
	// GVR for the managed object
	GroupVersionResource() *schema.GroupVersionResource
	// Kind for the managed object
	Kind() string

	Reconcile(obj metav1.Object) *hookv1.HookResponse
}

// HandlerFinalizable exposes methods for hook handler finalize operation.
type HandlerFinalizable interface {
	Finalize(obj metav1.Object) hookv1.HookResponse
}
