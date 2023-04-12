package kubernetes

import (
	"os"
	"path/filepath"

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

func NewClient(path string) (kclient.Interface, error) {
	cfg, err := NewConfig(path)
	if err != nil {
		return nil, err
	}

	return kclient.NewForConfig(cfg)
}
