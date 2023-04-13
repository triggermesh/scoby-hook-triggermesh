// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Registry map[schema.GroupVersionKind]Handler

func NewRegistry(h []Handler) Registry {
	r := make(map[schema.GroupVersionKind]Handler, len(h))

	for i := range h {
		gvr := h[i].GroupVersionResource()
		r[schema.GroupVersionKind{
			Group:   gvr.Group,
			Version: gvr.Version,
			Kind:    h[i].Kind(),
		}] = h[i]
	}

	return r
}
