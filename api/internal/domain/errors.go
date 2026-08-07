package domain

import "errors"

var (
	ErrAppNotFound        = errors.New("app not found")
	ErrAppNameTaken       = errors.New("app name already in use")
	ErrAppNameInvalid     = errors.New("app name is invalid")
	ErrDeploymentNotFound = errors.New("deployment not found")
	ErrBuildFailed        = errors.New("build failed")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrEnvKeyInvalid      = errors.New("env key is invalid")
	ErrEnvVarNotFound     = errors.New("env var not found")
)
