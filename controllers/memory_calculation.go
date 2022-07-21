package controllers

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"math"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func calculateNumberOfReplicas(c client.Client, size resource.Quantity) (int, error) {
	nodes := &v1.NodeList{}
	if err := c.List(context.TODO(), nodes); err != nil {
		return 0, err
	}

	if size.IsZero() {
		size = resource.MustParse("1Gi")
	}
	userSpecifiedSize := size.AsApproximateFloat64()

	smallest := math.MaxFloat64
	for _, n := range nodes.Items {
		amount := n.Status.Capacity.Memory().AsApproximateFloat64()
		if amount < smallest {
			smallest = amount
		}
	}
	maxNumberOfPods := math.RoundToEven(smallest/userSpecifiedSize) - 1

	//safety net for testing
	if maxNumberOfPods < 3 {
		return 3, nil
	}
	if maxNumberOfPods > 7 {
		return 7, nil
	}

	return int(maxNumberOfPods), nil
}
