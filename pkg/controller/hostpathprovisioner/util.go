/*
Copyright 2019 The hostpath provisioner operator Authors.

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

package hostpathprovisioner

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"reflect"

	jsondiff "github.com/appscode/jsonpatch"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	createVersionLabel          = "hostpathprovisioner.kubevirt.io/createVersion"
	updateVersionLabel          = "hostpathprovisioner.kubevirt.io/updateVersion"
	lastAppliedConfigAnnotation = "hostpathprovisioner.kubevirt.io/lastAppliedConfiguration"
)

func mergeLabelsAndAnnotations(src, dest metav1.Object) {
	// allow users to add labels but not change ours
	for k, v := range src.GetLabels() {
		if dest.GetLabels() == nil {
			dest.SetLabels(map[string]string{})
		}

		dest.GetLabels()[k] = v
	}

	// same for annotations
	for k, v := range src.GetAnnotations() {
		if dest.GetAnnotations() == nil {
			dest.SetAnnotations(map[string]string{})
		}

		dest.GetAnnotations()[k] = v
	}
}

func mergeObject(desiredObj, currentObj client.Object) (client.Object, error) {
	desiredMetaObj := desiredObj.(metav1.Object)
	currentMetaObj := currentObj.(metav1.Object)

	v, ok := currentMetaObj.GetAnnotations()[lastAppliedConfigAnnotation]
	if !ok {
		return nil, fmt.Errorf("%T %s/%s missing last applied config",
			currentMetaObj, currentMetaObj.GetNamespace(), currentMetaObj.GetName())
	}

	original := []byte(v)

	// setting the timestamp saves unnecessary updates because creation timestamp is nulled
	desiredMetaObj.SetCreationTimestamp(currentMetaObj.GetCreationTimestamp())
	modified, err := json.Marshal(desiredObj)
	if err != nil {
		return nil, err
	}

	current, err := json.Marshal(currentObj)
	if err != nil {
		return nil, err
	}

	preconditions := []mergepatch.PreconditionFunc{
		mergepatch.RequireKeyUnchanged("apiVersion"),
		mergepatch.RequireKeyUnchanged("kind"),
		mergepatch.RequireMetadataKeyUnchanged("name"),
	}

	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(original, modified, current, preconditions...)
	if err != nil {
		return nil, err
	}

	newCurrent, err := jsonpatch.MergePatch(current, patch)
	if err != nil {
		return nil, err
	}

	result := newDefaultInstance(currentObj)
	if err = json.Unmarshal(newCurrent, result); err != nil {
		return nil, err
	}

	return result, nil
}

func logJSONDiff(logger logr.Logger, objA, objB interface{}) {
	aBytes, _ := json.Marshal(objA)
	bBytes, _ := json.Marshal(objB)
	patches, _ := jsondiff.CreatePatch(aBytes, bBytes)
	pBytes, _ := json.Marshal(patches)
	logger.Info("DIFF", "obj", objA, "patch", string(pBytes))
}

func newDefaultInstance(obj client.Object) client.Object {
	typ := reflect.ValueOf(obj).Elem().Type()
	return reflect.New(typ).Interface().(client.Object)
}

func setLastAppliedConfiguration(obj metav1.Object) error {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(make(map[string]string))
	}

	obj.GetAnnotations()[lastAppliedConfigAnnotation] = string(bytes)

	return nil
}

func getResourceNameWithMaxLength(base, suffix string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	name := fmt.Sprintf("%s-%s", base, suffix)
	if len(name) <= maxLength {
		return name
	}

	baseLength := maxLength - 10 /*length of -hash-*/ - len(suffix)

	// if the suffix is too long, ignore it
	if baseLength < 0 {
		prefix := base[0:min(len(base), max(0, maxLength-9))]
		// Calculate hash on initial base-suffix string
		shortName := fmt.Sprintf("%s-%s", prefix, hash(name))
		return shortName[:min(maxLength, len(shortName))]
	}

	prefix := base[0:baseLength]
	// Calculate hash on initial base-suffix string
	return fmt.Sprintf("%s-%s-%s", prefix, hash(base), suffix)
}

// hash calculates the hexadecimal representation (8-chars)
// of the hash of the passed in string using the FNV-a algorithm
func hash(s string) string {
	hash := fnv.New32a()
	hash.Write([]byte(s))
	intHash := hash.Sum32()
	result := fmt.Sprintf("%08x", intHash)
	return result
}

// max returns the greater of its 2 inputs
func max(a, b int) int {
	if b > a {
		return b
	}
	return a
}

// min returns the lesser of its 2 inputs
func min(a, b int) int {
	if b < a {
		return b
	}
	return a
}
