package annotations

import (
	"context"
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Versioned interface {
	client.Object
	GetMongoDBVersionForAnnotation() string
	NamespacedName() types.NamespacedName
	IsChangingVersion() bool
}

type patchValue struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

const (
	LastAppliedMongoDBVersion = "mongodb.com/v1.lastAppliedMongoDBVersion"
)

func GetAnnotation(object client.Object, key string) string {
	value, ok := object.GetAnnotations()[key]
	if !ok {
		return ""
	}
	return value
}

func SetAnnotations(object client.Object, annotations map[string]string, kubeClient client.Client) error {
	currentObject := object
	err := kubeClient.Get(context.TODO(), types.NamespacedName{Name: object.GetName(), Namespace: object.GetNamespace()}, currentObject)
	if err != nil {
		return err
	}

	// If the object has no annotations, we first need to create an empty entry in
	// metadata.annotations, otherwise the server will reject our request
	var payload []patchValue
	if currentObject.GetAnnotations() == nil || len(currentObject.GetAnnotations()) == 0 {
		payload = append(payload, patchValue{
			Op:    "replace",
			Path:  "/metadata/annotations",
			Value: map[string]interface{}{},
		})
	}

	for key, val := range annotations {
		payload = append(payload, patchValue{
			Op: "replace",
			// every "/" in the value needs to be replaced with ~1 when patching
			Path:  "/metadata/annotations/" + strings.Replace(key, "/", "~1", 1),
			Value: val,
		})
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	patch := client.RawPatch(types.JSONPatchType, data)
	return kubeClient.Patch(context.TODO(), object, patch)
}

func UpdateLastAppliedMongoDBVersion(mdb Versioned, kubeClient client.Client) error {
	annotations := map[string]string{
		LastAppliedMongoDBVersion: mdb.GetMongoDBVersionForAnnotation(),
	}

	return SetAnnotations(mdb, annotations, kubeClient)
}

