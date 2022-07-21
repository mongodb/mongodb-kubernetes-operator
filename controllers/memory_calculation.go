package controllers

import "sigs.k8s.io/controller-runtime/pkg/client"

func calculateNumberOfReplicas(c client.Client) (uint8, error) {
	return 1, nil
}
