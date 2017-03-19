package main

import (
	"flag"
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/golang/glog"

	"kope.io/auth/pkg/apis/componentconfig"
	"kope.io/auth/pkg/configreader"
	"kope.io/auth/pkg/keystore"
	"kope.io/auth/pkg/portal"

	cryptorand "crypto/rand"
	"encoding/binary"
	"io/ioutil"
	mathrand "math/rand"

	"kope.io/auth/pkg/api"

	componentconfiginstall "kope.io/auth/pkg/apis/componentconfig/install"
)

const CookieSigningSecretLength = 24

func main() {
	cryptoSeed()

	flag.Set("logtostderr", "true")

	// TODO(componentconfig-q): Some parameters we don't really want configurable, because
	// we expect to be running in a container.  But maybe they would be useful for people
	// that want to run the code differently, so they probably warrant a flag or an env var.
	// Thoughts?
	listen := ":8080"
	flag.StringVar(&listen, "listen", listen, "host/port on which to listen")
	staticDir := "/webapp"
	flag.StringVar(&staticDir, "static-dir", staticDir, "location of static directory")

	flag.Parse()

	err := run(listen, staticDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unexpected error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(listen string, staticDir string) error {
	k8sClient, err := buildK8sClient()
	if err != nil {
		return err
	}

	name := "auth"

	namespaceBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return fmt.Errorf("error reading namespace from %q: %v", "/var/run/secrets/kubernetes.io/serviceaccount/namespace", err)
	}
	namespace := string(namespaceBytes)

	apiContext, err := api.NewAPIContext(os.Getenv("API_VERSIONS"))
	if err != nil {
		return fmt.Errorf("error initializing API: %v", err)
	}

	componentconfiginstall.Install(apiContext.GroupFactoryRegistry, apiContext.Registry, apiContext.Scheme)

	configDecoder := apiContext.Codecs.UniversalDecoder()

	configReader := &configreader.ManagedConfiguration{
		Decoder: configDecoder,
	}

	//configFile := os.Getenv("CONFIG")
	//if configFile != "" {
	//	err := configReader.Read(configFile)
	//	if err != nil {
	//		return fmt.Errorf("error reading config file %q: %v\n", configFile, err)
	//	}
	//}

	configObj, err := configReader.ReadFromKubernetes(k8sClient, namespace, name)
	if err != nil {
		return fmt.Errorf("error reading configuration: %v", err)
	}

	// TODO(componentconfig-q): Should we deal with v1alpha1 or unversioned when we own the API?
	// (I guess the same question with our User objects)
	config := configObj.(*componentconfig.AuthConfiguration)

	secretStore, err := keystore.NewKubernetesKeyStore(k8sClient, namespace, name)
	if err != nil {
		return err
	}
	stopCh := make(chan struct{})
	go secretStore.Run(stopCh)

	sharedSecretSet, err := secretStore.EnsureSharedSecretSet("cookie-signing", generateCookieSigningSecrets)
	if err != nil {
		return err
	}

	//o.ClientID = os.Getenv("OAUTH2_CLIENT_ID")
	//o.ClientSecret = os.Getenv("OAUTH2_CLIENT_SECRET")
	//o.CookieSecret = os.Getenv("OAUTH2_COOKIE_SECRET")

	p, err := portal.NewHTTPServer(config, listen, staticDir, sharedSecretSet)
	if err != nil {
		return err
	}

	return p.ListenAndServe()
}

func generateCookieSigningSecrets() ([]byte, error) {
	data := make([]byte, CookieSigningSecretLength)
	_, err := cryptorand.Read(data)
	if err != nil {
		return nil, fmt.Errorf("error generating cookie signing secret: %v", err)
	}
	return data, nil
}

func cryptoSeed() {
	data := make([]byte, 8)
	_, err := cryptorand.Read(data)
	if err != nil {
		glog.Fatalf("error seeding random numbers: %v", err)
	}
	seed := binary.BigEndian.Uint64(data)
	mathrand.Seed(int64(seed))
}

func buildK8sClient() (kubernetes.Interface, error) {
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes client configuration: %v", err)
	}
	// creates the clientset
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error building kubernetes client: %v", err)
	}
	return client, err
}
