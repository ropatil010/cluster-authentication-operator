package library

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	imagev1 "github.com/openshift/api/image/v1"
)

var imageStreamGVR = schema.GroupVersionResource{
	Group:    "image.openshift.io",
	Version:  "v1",
	Resource: "imagestreams",
}

// ImportImageToImageStream imports an external image into a namespace's ImageStream.
// This ensures the image is pulled from the cluster's internal registry, which is allowed
// by the known-image-checker monitor test.
//
// Parameters:
//   - registry: image registry (e.g., "quay.io", "docker.io")
//   - name: image name (e.g., "keycloak/keycloak", "gitlab/gitlab-ce")
//   - version: image tag/version (e.g., "25.0", "13.8.4-ce.0")
//   - imageStreamName: the name for the ImageStream (e.g., "keycloak", "gitlab")
//
// Returns:
//   - internalImage: the internal registry reference that pods should use
//   - cleanup: function to delete the ImageStream
//   - error: any error that occurred
func ImportImageToImageStream(
	t testing.TB,
	kubeconfig *rest.Config,
	namespace string,
	registry string,
	name string,
	version string,
	imageStreamName string,
) (string, func(), error) {
	dynamicClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Construct the full source image reference: registry/name:version
	sourceImage := fmt.Sprintf("%s/%s:%s", registry, name, version)

	// Normalize registry (docker.io uses library/ prefix for official images if no org specified)
	if registry == "docker.io" && !strings.Contains(name, "/") {
		sourceImage = fmt.Sprintf("%s/library/%s:%s", registry, name, version)
	}

	t.Logf("Importing image %s into ImageStream %s/%s with tag %s", sourceImage, namespace, imageStreamName, version)

	// Create ImageStream using the typed API structure
	is := &imagev1.ImageStream{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "image.openshift.io/v1",
			Kind:       "ImageStream",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      imageStreamName,
			Namespace: namespace,
		},
		Spec: imagev1.ImageStreamSpec{
			Tags: []imagev1.TagReference{
				{
					Name: version,
					From: &corev1.ObjectReference{
						Kind: "DockerImage",
						Name: sourceImage,
					},
					ImportPolicy: imagev1.TagImportPolicy{
						Scheduled: false, // Don't auto-reimport
					},
					ReferencePolicy: imagev1.TagReferencePolicy{
						Type: imagev1.LocalTagReferencePolicy, // Use local reference for pulls
					},
				},
			},
		},
	}

	// Convert to unstructured
	unstructuredIS, err := toUnstructured(is)
	if err != nil {
		return "", nil, fmt.Errorf("failed to convert ImageStream to unstructured: %w", err)
	}

	// Create the ImageStream
	_, err = dynamicClient.Resource(imageStreamGVR).Namespace(namespace).Create(
		context.TODO(), unstructuredIS, metav1.CreateOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("failed to create imagestream: %w", err)
	}

	cleanup := func() {
		if err := dynamicClient.Resource(imageStreamGVR).Namespace(namespace).Delete(
			context.TODO(), imageStreamName, metav1.DeleteOptions{}); err != nil {
			t.Logf("failed to delete imagestream %s/%s: %v", namespace, imageStreamName, err)
		}
	}

	// Wait for the image to be imported
	t.Logf("Waiting for image to be imported into ImageStream %s/%s...", namespace, imageStreamName)
	err = wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		isUnstructured, err := dynamicClient.Resource(imageStreamGVR).Namespace(namespace).Get(
			ctx, imageStreamName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Convert back to typed ImageStream to check status
		var is imagev1.ImageStream
		if err := fromUnstructured(isUnstructured, &is); err != nil {
			return false, err
		}

		// Check if the tag has been imported (has a dockerImageReference)
		for _, tag := range is.Status.Tags {
			if tag.Tag == version && len(tag.Items) > 0 && tag.Items[0].DockerImageReference != "" {
				t.Logf("Image successfully imported: %s", tag.Items[0].DockerImageReference)
				return true, nil
			}
		}
		return false, nil
	})

	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("timeout waiting for image import: %w", err)
	}

	// Return the internal registry reference
	// Format: image-registry.openshift-image-registry.svc:5000/<namespace>/<imagestream>:<tag>
	internalImage := fmt.Sprintf("image-registry.openshift-image-registry.svc:5000/%s/%s:%s",
		namespace, imageStreamName, version)

	t.Logf("Image available at internal registry: %s", internalImage)
	return internalImage, cleanup, nil
}

// toUnstructured converts a runtime.Object to unstructured
func toUnstructured(obj interface{}) (*unstructured.Unstructured, error) {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: unstructuredObj}, nil
}

// fromUnstructured converts an unstructured object to a typed object
func fromUnstructured(u *unstructured.Unstructured, obj interface{}) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj)
}
