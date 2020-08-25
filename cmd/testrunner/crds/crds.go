package crds

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/ghodss/yaml"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
)

// EnsureCreation will locate all crd files "*_crd.yaml" in the given deploy directory and ensure that these
// CRDs are created into the kubernetes cluster
func EnsureCreation(config *rest.Config, deployDir string) error {
	apiextensionsClientSet, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return errors.Errorf("error creating apiextensions client set: %s", err)
	}

	crdFilePaths, err := allCrds(deployDir)
	if err != nil {
		return errors.Errorf("error walking deploy directory: %s", err)
	}

	for _, filePath := range crdFilePaths {
		crd := &apiextensionsv1beta1.CustomResourceDefinition{}
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			return errors.Errorf("error reading file: %s", err)
		}
		if err := marshalCRDFromYAMLBytes(data, crd); err != nil {
			return errors.Errorf("error converting yaml bytes to CRD: %s", err)
		}
		_, err = apiextensionsClientSet.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)

		if apierrors.IsAlreadyExists(err) {
			fmt.Println("CRD already exists")
			continue
		}

		if err != nil {
			return errors.Errorf("error creating custom resource definition: %s", err)
		}
	}
	return nil
}

func marshalCRDFromYAMLBytes(bytes []byte, crd *apiextensionsv1beta1.CustomResourceDefinition) error {
	jsonBytes, err := yaml.YAMLToJSON(bytes)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonBytes, &crd)
}

func allCrds(deployDir string) ([]string, error) {
	crdDir := path.Join(deployDir, "crds")
	var crdFilePaths []string
	err := filepath.Walk(crdDir, func(path string, info os.FileInfo, err error) error {
		if info != nil && strings.HasSuffix(info.Name(), "_crd.yaml") {
			fmt.Printf("Found CRD: %s\n", info.Name())
			crdFilePaths = append(crdFilePaths, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return crdFilePaths, nil
}
