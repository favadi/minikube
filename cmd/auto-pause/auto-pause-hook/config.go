/*
This file is a modified version of
https://github.com/caesarxuchao/example-webhook-admission-controller/blob/master/config.go
*/

/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/golang/glog"
)

// get a clientset with in-cluster config.
func getClient() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatal(err)
	}
	return clientset
}

// retrieve the CA cert that will signed the cert used by the
// "GenericAdmissionWebhook" plugin admission controller.
func getAPIServerCert(clientset *kubernetes.Clientset) []byte {
	c, err := clientset.CoreV1().ConfigMaps("kube-system").Get("extension-apiserver-authentication", metav1.GetOptions{})
	if err != nil {
		glog.Fatal(err)
	}

	pem, ok := c.Data["requestheader-client-ca-file"]
	if !ok {
		glog.Fatalf(fmt.Sprintf("cannot find the ca.crt in the configmap, configMap.Data is %#v", c.Data))
	}
	glog.Info("client-ca-file=", pem)
	return []byte(pem)
}

func configTLS(clientset *kubernetes.Clientset, serverCert []byte, serverKey []byte) *tls.Config {
	cert := getAPIServerCert(clientset)
	apiserverCA := x509.NewCertPool()
	apiserverCA.AppendCertsFromPEM(cert)

	sCert, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		glog.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{sCert},
		ClientCAs:    apiserverCA,
		ClientAuth:   tls.VerifyClientCertIfGiven, // TODO: actually require client cert
	}
}

// register this example webhook admission controller with the kube-apiserver
// by creating externalAdmissionHookConfigurations.
func selfRegistration(clientset *kubernetes.Clientset, caCert []byte) {
	time.Sleep(10 * time.Second)
	client := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations()
	_, err := client.Get("env-inject-webhook", metav1.GetOptions{})
	if err == nil {
		if err2 := client.Delete("env-inject-webhook", &metav1.DeleteOptions{}); err2 != nil {
			glog.Fatal(err2)
		}
	}
	var failurePolicy v1.FailurePolicyType = v1.Fail
	var sideEffects v1.SideEffectClass = v1.SideEffectClassNone

	webhookConfig := &v1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "env-inject-webhook",
		},
		Webhooks: []v1.MutatingWebhook{
			{
				Name: "env-inject.zyanshu.io",
				Rules: []v1.RuleWithOperations{
					{
						Operations: []v1.OperationType{v1.Create, v1.Update},
						Rule: v1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
					{
						Operations: []v1.OperationType{v1.Create, v1.Update},
						Rule: v1.Rule{
							APIGroups:   []string{"extensions"},
							APIVersions: []string{"v1"},
							Resources:   []string{"deployments"},
						},
					},
				},
				FailurePolicy: &failurePolicy,
				ClientConfig: v1.WebhookClientConfig{
					Service: &v1.ServiceReference{
						Namespace: "auto-pause",
						Name:      "webhook",
					},
					CABundle: caCert,
				},
				AdmissionReviewVersions: []string{"v1"},
				SideEffects:             &sideEffects,
			},
		},
	}
	if _, err := client.Create(webhookConfig); err != nil {
		glog.Fatalf("Client creation failed with %s", err)
	}
	log.Println("CLIENT CREATED")
}
