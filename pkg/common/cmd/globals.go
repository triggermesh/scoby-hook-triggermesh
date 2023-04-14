// Copyright 2023 TriggerMesh Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"go.uber.org/automaxprocs/maxprocs"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kdclient "k8s.io/client-go/dynamic"
	kclient "k8s.io/client-go/kubernetes"

	"github.com/triggermesh/scoby-hook-triggermesh/pkg/config/observability"
	"github.com/triggermesh/scoby-hook-triggermesh/pkg/kubernetes"
)

type ConfigMethod int

const (
	ConfigMethodUnknown = iota
	ConfigMethodKubernetesConfigMap
	ConfigMethodCommandLineOrEnv
)

type Globals struct {
	Port int `help:"HTTP Port to listen for hook requests." env:"PORT" default:"8080"`

	Kubeconfig string `help:"Kubeconfig file." env:"KUBECONFIG"`

	ObservabilityConfig string `help:"JSON representation of observability configuration." env:"OBSERVABILITY_CONFIG"`

	// Kubernetes parameters
	KubernetesNamespace                  string `help:"Namespace where the hook is running." env:"KUBERNETES_NAMESPACE"`
	KubernetesObservabilityConfigMapName string `help:"ConfigMap object name that contains the observability configuration." env:"KUBERNETES_OBSERVABILITY_CONFIGMAP_NAME"`

	Context      context.Context    `kong:"-"`
	Logger       *zap.SugaredLogger `kong:"-"`
	LogLevel     zap.AtomicLevel    `kong:"-"`
	KubeClient   kclient.Interface  `kong:"-"`
	DynClient    kdclient.Interface `kong:"-"`
	ConfigMethod ConfigMethod       `kong:"-"`
}

func (g *Globals) Validate() error {
	msg := []string{}

	switch {
	case g.KubernetesObservabilityConfigMapName != "":
		if g.KubernetesNamespace == "" {
			msg = append(msg, "Kubernetes namespace must be informed.")
		}

		if g.ObservabilityConfig != "" {
			msg = append(msg, "Argument or environment config cannot be used along with Kubernetes configuration.")
		}

		g.ConfigMethod = ConfigMethodKubernetesConfigMap

	case g.ObservabilityConfig != "":
		if g.KubernetesObservabilityConfigMapName != "" {
			msg = append(msg, "Argument or environment config cannot be used along with Kubernetes configuration.")
		}

		g.ConfigMethod = ConfigMethodCommandLineOrEnv
	}

	if len(msg) != 0 {
		g.ConfigMethod = ConfigMethodUnknown
		return fmt.Errorf(strings.Join(msg, " "))
	}

	return nil
}

func (g *Globals) Initialize() error {
	var cfg *observability.Config
	var l *zap.Logger
	defaultConfigApplied := false
	var err error

	undo, err := maxprocs.Set()
	if err != nil {
		return fmt.Errorf("could not match available CPUs to processes %w", err)
	}
	defer undo()

	kc, kdc, err := kubernetes.NewClients(g.Kubeconfig)
	if err != nil {
		return err
	}
	g.KubeClient, g.DynClient = kc, kdc

	switch {
	case g.ObservabilityConfig != "":
		data := map[string]string{}
		err = json.Unmarshal([]byte(g.ObservabilityConfig), &data)
		if err != nil {
			log.Printf("Could not appliying provided config: %v", err)
			defaultConfigApplied = true
			break
		}

		cfg, err = observability.ParseFromMap(data)
		if err != nil || cfg.LoggerCfg == nil {
			log.Printf("Could not appliying provided config: %v", err)
			defaultConfigApplied = true
		}

	case g.KubernetesObservabilityConfigMapName != "":
		cm := &corev1.ConfigMap{}
		var lastErr error
		if err := wait.PollImmediate(1*time.Second, 5*time.Second, func() (bool, error) {
			cm, lastErr = g.KubeClient.CoreV1().ConfigMaps(g.KubernetesNamespace).Get(g.Context, g.KubernetesObservabilityConfigMapName, metav1.GetOptions{})
			return lastErr == nil || apierrors.IsNotFound(lastErr), nil
		}); err != nil {
			log.Printf("Could not retrieve observability ConfigMap %q: %v",
				g.KubernetesObservabilityConfigMapName, err)
			defaultConfigApplied = true
		}

		cfg, err = observability.ParseFromMap(cm.Data)
		if err != nil || cfg.LoggerCfg == nil {
			log.Printf("Could not apply provided config from ConfigMap %q: %v",
				g.KubernetesObservabilityConfigMapName, err)
			defaultConfigApplied = true
		}

	default:
		log.Print("Applying default configuration")
		defaultConfigApplied = true
	}

	if defaultConfigApplied {
		cfg = observability.DefaultConfig()
	}

	// Call build to perform validation of zap configuration.
	l, err = cfg.LoggerCfg.Build()
	for {
		if err == nil {
			break
		}
		if defaultConfigApplied {
			return fmt.Errorf("default config failed to be applied due to error: %w", err)
		}

		defaultConfigApplied = true
		cfg = observability.DefaultConfig()
		l, err = cfg.LoggerCfg.Build()
	}

	g.LogLevel = cfg.LoggerCfg.Level

	g.Logger = l.Sugar()
	g.LogLevel = cfg.LoggerCfg.Level

	return nil
}

func (g *Globals) Flush() {
	if g.Logger != nil {
		_ = g.Logger.Sync()
	}
}
