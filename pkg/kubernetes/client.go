package kubernetes

import (
	"os"
	"path/filepath"

	kdclient "k8s.io/client-go/dynamic"
	kclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func NewConfig(path string) (*restclient.Config, error) {
	if path == "" {
		if home := homedir.HomeDir(); home != "" {
			path = filepath.Join(home, ".kube", "config")
		}

		// If path was not provided when calling this function, do not default to
		// a kubeconfig that does not exist.
		if _, err := os.Stat(path); err != nil {
			path = ""
		}
	}

	if path != "" {
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{ExplicitPath: path}, &clientcmd.ConfigOverrides{}).ClientConfig()
	}

	return restclient.InClusterConfig()
}

func NewClients(path string) (kclient.Interface, kdclient.Interface, error) {
	cfg, err := NewConfig(path)
	if err != nil {
		return nil, nil, err
	}

	kc, err := kclient.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	kdc, err := kdclient.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}

	return kc, kdc, err
}
