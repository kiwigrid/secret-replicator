package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	//"k8s.io/client-go/tools/cache"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var VERSION = "latest"

func main() {
	app := cli.NewApp()
	app.Version = VERSION
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config",
			Usage: "Kube config path for outside of cluster access",
		},
	}

	app.Action = func(c *cli.Context) error {
		var err error
		if err != nil {
			logrus.Error(err)
			return err
		}
		configFile := c.String("config")
		secrets := strings.Split(os.Getenv("PULL_SECRETS"), ",")

		var currentNamespace string
		if os.Getenv("SECRET_NAMESPACE") == "" {
			file, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
			if err != nil {
				logrus.Fatalf("Err: %v", err)
			}
			currentNamespace = string(file)
		} else {
			currentNamespace = os.Getenv("SECRET_NAMESPACE")
		}

		go startWatchSecrets(configFile, secrets, currentNamespace)
		go startWatchNamespaces(configFile, secrets, currentNamespace)
		for {
			time.Sleep(5 * time.Second)
		}
	}
	app.Run(os.Args)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func startWatchSecrets(pathToCfg string, secrets []string, lookupNamespace string) {
	clientSet, _ := getClient(pathToCfg)
	listOptions := metav1.ListOptions{}
	watcher, err := clientSet.CoreV1().Secrets(lookupNamespace).Watch(listOptions)
	if err != nil {
		logrus.Errorf("%v", err)
	}
	ch := watcher.ResultChan()

	for event := range ch {
		secret, ok := event.Object.(*v1.Secret)
		if !ok {
			logrus.Fatal("unexpected type")
			continue
		}

		if contains(secrets, secret.Name) {
			listOptions := metav1.ListOptions{}
			namespaces, err := clientSet.CoreV1().Namespaces().List(listOptions)
			if err != nil {
				logrus.Errorf("%v", err)
				continue
			}

			for _, ns := range namespaces.Items {
				if ns.Name == lookupNamespace {
					continue
				}
				createOrUpdateSecret(clientSet, secret, ns.Name, secret.Name)
			}
		}
	}
}

func startWatchNamespaces(pathToCfg string, secrets []string, lookupNamespace string) {
	clientSet, _ := getClient(pathToCfg)
	listOptions := metav1.ListOptions{}

	//listOptions := metav1.ListOptions{LabelSelector: "pull-secret-inject=enabled"}

	watcher, err := clientSet.CoreV1().Namespaces().Watch(listOptions)
	if err != nil {
		logrus.Errorf("%v", err)
	}
	ch := watcher.ResultChan()

	for event := range ch {
		namespace, ok := event.Object.(*v1.Namespace)
		if !ok {
			logrus.Fatal("unexpected type")
			continue
		}

		switch event.Type {
		case watch.Added:
			{
				handleNamespace(clientSet, lookupNamespace, namespace.Name, secrets)
				logrus.Infof("add namespace %v handled", namespace.Name)
			}
		}
	}
}

func handleNamespace(clientSet *kubernetes.Clientset, currentNamespace string, namespace string, secrets []string) {
	getOptions := metav1.GetOptions{}
	for _, element := range secrets {
		if element == "" {
			continue
		}
		copySecret, err := clientSet.CoreV1().Secrets(currentNamespace).Get(element, getOptions)
		if err != nil {
			logrus.Errorf("Could not find secret %v in namespace %v", element, currentNamespace)
			continue
		}
		createOrUpdateSecret(clientSet, copySecret, namespace, copySecret.Name)
	}
}

func createOrUpdateSecret(clientSet *kubernetes.Clientset, copySecret *v1.Secret, namespace string, secretName string) {
	getOptions := metav1.GetOptions{}
	secret, err := clientSet.CoreV1().Secrets(namespace).Get(secretName, getOptions)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logrus.Infof("secret %v does not exists in namespace %v creating new", secretName, namespace)
			kubSecret := &v1.Secret{}
			kubSecret.Name = copySecret.Name
			kubSecret.Type = copySecret.Type
			kubSecret.Data = map[string][]byte{}
			kubSecret.Data[".dockercfg"] = copySecret.Data[".dockercfg"]
			_, err = clientSet.CoreV1().Secrets(namespace).Create(kubSecret)
			if err == nil {
				logrus.Infof("successful saved secret %v in namespace %v", secretName, namespace)
			} else {
				logrus.Errorf("%v", err)
			}
		} else {
			logrus.Errorf("%v", err)
		}
	} else {
		secret.Data[".dockercfg"] = copySecret.Data[".dockercfg"]
		_, err = clientSet.CoreV1().Secrets(namespace).Update(secret)
		if err == nil {
			logrus.Infof("successful updated secret %v in namespace %v", secretName, namespace)
		} else {
			logrus.Errorf("%v", err)
		}
	}
}

func getClient(pathToCfg string) (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if pathToCfg == "" {
		// in cluster access
		logrus.Info("Using in cluster config")
		config, err = rest.InClusterConfig()
	} else {
		logrus.Info("Using out of cluster config")
		config, err = clientcmd.BuildConfigFromFlags("", pathToCfg)
	}
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)

}
