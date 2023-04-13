// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kdclient "k8s.io/client-go/dynamic"

	"github.com/triggermesh/scoby-hook-triggermesh/pkg/handler"
	hookv1 "github.com/triggermesh/scoby/pkg/hook/v1"
)

type Server struct {
	path    string
	address string
	reg     handler.Registry

	dyn kdclient.Interface

	logger *zap.SugaredLogger `kong:"-"`
}

func New(path, address string, reg handler.Registry, dyn kdclient.Interface, logger *zap.SugaredLogger) *Server {
	return &Server{
		path:    path,
		address: address,
		reg:     reg,
		dyn:     dyn,

		logger: logger,
	}

}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/v1", s)

	srv := http.Server{
		Addr:    s.address,
		Handler: mux,
	}

	errCh := make(chan error)

	go func() {
		s.logger.Infow("Starting TriggerMesh Scoby webhook", zap.String("address", s.address))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// pods, err := s.client.CoreV1().Pods("default").List(ctx, metav1.ListOptions{})
	// if err != nil {
	// 	return err
	// }

	// for _, p := range pods.Items {
	// 	s.logger.Infow("pod", zap.String("name", p.Name))
	// }

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()

		srv.Shutdown(ctx)
	}

	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hreq := &hookv1.HookRequest{}
	if err := json.NewDecoder(r.Body).Decode(hreq); err != nil {
		msg := "cannot decode request into HookRequest: " + err.Error()
		s.logger.Error("Error decoding incoming request", zap.Error(errors.New(msg)))
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	s.logger.Debug("Received request", zap.Any("request", hreq))

	gv, err := schema.ParseGroupVersion(hreq.Object.APIVersion)
	if err != nil {
		msg := "cannot parse APIVersion from HookRequest: " + err.Error()
		s.logger.Error("Error parsing HookRequest", zap.Error(errors.New(msg)))
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	gvk := gv.WithKind(hreq.Object.Kind)
	h, ok := s.reg[gvk]
	if !ok {
		msg := fmt.Sprintf("the hook does not contain a handler for %q", gvk.String())
		s.logger.Error("Error serving HookRequest", zap.Error(errors.New(msg)))
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	obj, err := s.dyn.Resource(*h.GroupVersionResource()).
		Namespace(hreq.Object.Namespace).
		Get(r.Context(), hreq.Object.Name, metav1.GetOptions{})
	if err != nil {
		msg := "object at the HookRequest cannot be found: " + err.Error()
		s.logger.Error("Error processing request", zap.Error(errors.New(msg)))

		// Using no content, to make clear that the API and the handler
		// for the registered object exists, but the object cannot be retrieved.
		http.Error(w, msg, http.StatusNoContent)
		return
	}

	var hres *hookv1.HookResponse
	switch hreq.Operation {
	case hookv1.OperationReconcile:
		hres = h.Reconcile(obj)

	case hookv1.OperationFinalize:
		f, ok := h.(handler.HandlerFinalizable)
		if !ok {
			msg := "hook handler does not support Finalizers"
			s.logger.Error("Error processing request", zap.Error(errors.New(msg)))
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		hres = f.Finalize(obj)

	default:
		msg := "request must be either " + string(hookv1.OperationReconcile) +
			" or " + string(hookv1.OperationFinalize)
		s.logger.Error("Error parsing request", zap.Error(errors.New(msg)))
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hres)
}
