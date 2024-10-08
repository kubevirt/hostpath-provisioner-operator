/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package v1beta1

import (
	"context"

	v1beta1 "k8s.io/api/authentication/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gentype "k8s.io/client-go/gentype"
	scheme "k8s.io/client-go/kubernetes/scheme"
)

// TokenReviewsGetter has a method to return a TokenReviewInterface.
// A group's client should implement this interface.
type TokenReviewsGetter interface {
	TokenReviews() TokenReviewInterface
}

// TokenReviewInterface has methods to work with TokenReview resources.
type TokenReviewInterface interface {
	Create(ctx context.Context, tokenReview *v1beta1.TokenReview, opts v1.CreateOptions) (*v1beta1.TokenReview, error)
	TokenReviewExpansion
}

// tokenReviews implements TokenReviewInterface
type tokenReviews struct {
	*gentype.Client[*v1beta1.TokenReview]
}

// newTokenReviews returns a TokenReviews
func newTokenReviews(c *AuthenticationV1beta1Client) *tokenReviews {
	return &tokenReviews{
		gentype.NewClient[*v1beta1.TokenReview](
			"tokenreviews",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *v1beta1.TokenReview { return &v1beta1.TokenReview{} }),
	}
}
