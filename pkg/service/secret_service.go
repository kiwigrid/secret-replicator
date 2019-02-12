package pullsecretservice

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"strings"
)

type PullSecretService struct {
	log logr.Logger
}

func NewPullSecretService() *PullSecretService {
	return &PullSecretService{log: logf.Log.WithName("pullsecretservice")}
}
func (s *PullSecretService) CheckServiceAccountExists(client client.Client, copySecret *corev1.Secret, namespace string, currentNamespace string, secrets []string) (bool, error) {

	for _, element := range secrets {
		if element == "" {
			continue
		}

		found := &corev1.Secret{}
		err := client.Get(context.TODO(), types.NamespacedName{Name: element, Namespace: namespace}, found)
		if err != nil {
			s.log.Error(err, "Could not find secret %v in namespace %v")
			continue
		}
		s.CreateOrUpdateSecret(client, copySecret, namespace, copySecret.Name)
	}
	return true, nil
}

func (s *PullSecretService) CreateOrUpdateSecret(client client.Client, copySecret *corev1.Secret, namespace string, secretName string) {
	secret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: secretName, Namespace: namespace}, secret)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			//logrus.Infof("secret %v does not exists in namespace %v creating new", secretName, namespace)
			kubSecret := &v1.Secret{}
			kubSecret.Namespace = namespace
			kubSecret.Name = copySecret.Name
			kubSecret.Type = copySecret.Type
			kubSecret.Data = map[string][]byte{}

			for k, v := range copySecret.Data {
				s.log.Info("copy %s for secret %s", k, copySecret.Name)
				kubSecret.Data[k] = v
			}

			err := client.Create(context.TODO(), kubSecret)
			if err == nil {
				s.log.Info(fmt.Sprintf("successful saved secret %v in namespace %v", secretName, namespace))
			} else {
				s.log.Error(err, "ERROR")
			}
		} else {
			s.log.Error(err, "ERROR")
		}
	} else {
		secret.Data[".dockercfg"] = copySecret.Data[".dockercfg"]
		err := client.Update(context.TODO(), secret)
		if err == nil {
			s.log.Info(fmt.Sprintf("successful updated secret %v in namespace %v", secretName, namespace))
		} else {
			s.log.Error(err, "ERROR")
		}
	}
}
