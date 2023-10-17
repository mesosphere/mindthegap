// Copyright 2021 D2iQ, Inc. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package authnhelpers

import (
	"github.com/containers/image/v5/types"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/google/go-containerregistry/pkg/authn"
)

type staticHelper struct {
	registry   string
	authConfig *types.DockerAuthConfig
}

var _ authn.Helper = staticHelper{}

func (h staticHelper) Get(serverURL string) (username, password string, err error) {
	if h.authConfig != nil && serverURL == h.registry {
		password := h.authConfig.Password
		if password == "" {
			password = h.authConfig.IdentityToken
		}
		return h.authConfig.Username, password, nil
	}

	return "", "", credentials.NewErrCredentialsNotFound()
}

func NewStaticHelper(registry string, authConfig *types.DockerAuthConfig) authn.Helper {
	return staticHelper{
		registry:   registry,
		authConfig: authConfig,
	}
}
