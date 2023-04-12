package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	commonv1alpha1 "github.com/triggermesh/scoby/pkg/apis/common/v1alpha1"
	hookv1 "github.com/triggermesh/scoby/pkg/hook/v1"
)

type Server struct {
	path    string
	address string

	logger   *zap.SugaredLogger `kong:"-"`
	handlers map[string]interface{}
}

func New(path, address string, logger *zap.SugaredLogger) *Server {
	return &Server{
		path:    path,
		address: address,

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

	// Is the object registered?

	// Retrieve object

	// Call function Reconcile/Finalize

	switch hreq.Operation {
	case hookv1.OperationReconcile:

		log.Printf("received reconcile request: %v\n", *hreq)
	case hookv1.OperationFinalize:
		log.Printf("received finalize request: %v\n", *hreq)
	default:
		msg := "request must be either " + string(hookv1.OperationReconcile) +
			" or " + string(hookv1.OperationFinalize)
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	hres := &hookv1.HookResponse{
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
				Name:  "FROM_HOOK",
				Value: "env from hook",
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hres)
}

type handlerIndex struct {
	apiVersion string
	kind       string
}

type Handler struct {
}

func (h *Handler) Reconcile(namespace, name string) {

}

func (h *Handler) Finalize(namespace, name string) {

}
