package main

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestContainsMethod(t *testing.T) {

	b := contains([]string{"foo", "bar"}, "foo")
	if b == false {
		t.Fatal("contains should return true")
	}

	b = contains([]string{"foo", "bar"}, "foo2")
	if b == true {
		t.Fatal("contains should return false")
	}
}

func TestHandleNamespace(t *testing.T) {
	clientSet := fake.NewSimpleClientset(
		&v1.NamespaceList{Items: []v1.Namespace{*NewNamespace("default"), *NewNamespace("test")}},
		&v1.SecretList{Items: []v1.Secret{*NewSecret("test_secret", "default")}},
	)
	handleNamespace(clientSet, "default", "test", []string{"test_secret"}, []string{})

	getOptions := metav1.GetOptions{}
	_, err := clientSet.CoreV1().Secrets("test").Get("test_secret", getOptions)
	if err != nil {
		t.Fatal(err.Error())
	}
}

func NewNamespace(name string) *v1.Namespace {
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.NamespaceSpec{
		},
		Status: v1.NamespaceStatus{
			Phase: "Active",
		},
	}

	return namespace
}

func NewSecret(name, namespace string) *v1.Secret {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
		},
	}

	return secret
}
