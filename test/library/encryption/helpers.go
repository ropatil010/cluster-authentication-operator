package encryption

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	kubecmdapi "k8s.io/client-go/tools/clientcmd/api"

	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
)

type ClientSet struct {
	OperatorClient operatorv1client.AuthenticationInterface
	TokenClient    oauthclient.OAuthAccessTokensGetter
}

func GetClientsFor(t testing.TB, kubeConfig *rest.Config) ClientSet {
	t.Helper()

	operatorClient, err := operatorv1client.NewForConfig(kubeConfig)
	require.NoError(t, err)

	oc, err := oauthclient.NewForConfig(kubeConfig)
	require.NoError(t, err)

	return ClientSet{OperatorClient: operatorClient.Authentications(), TokenClient: oc}
}

func GetClients(t testing.TB) ClientSet {
	t.Helper()

	kubeConfig := NewClientConfigForTest(t)

	return GetClientsFor(t, kubeConfig)
}

// NewClientConfigForTest returns a config configured to connect to the api server
func NewClientConfigForTest(t testing.TB) *rest.Config {
	loader := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loader, &clientcmd.ConfigOverrides{ClusterInfo: kubecmdapi.Cluster{InsecureSkipTLSVerify: true}})
	config, err := clientConfig.ClientConfig()
	if err == nil {
		fmt.Printf("Found configuration for host %v.\n", config.Host)
	}
	require.NoError(t, err)
	return config
}
