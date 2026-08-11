package domain

import "errors"

var (
	ErrAppNotFound               = errors.New("app not found")
	ErrAppNameTaken              = errors.New("app name already in use")
	ErrAppNameInvalid            = errors.New("app name is invalid")
	ErrDeploymentNotFound        = errors.New("deployment not found")
	ErrDeploymentActive          = errors.New("deployment is already active")
	ErrDeploymentNotRollbackable = errors.New("deployment is not eligible for rollback")
	ErrBuildFailed               = errors.New("build failed")
	ErrInvalidCredentials        = errors.New("invalid credentials")
	ErrUnauthorized              = errors.New("unauthorized")
	ErrEnvKeyInvalid             = errors.New("env key is invalid")
	ErrEnvVarNotFound            = errors.New("env var not found")
	ErrGitSourceNotFound         = errors.New("git source not configured")
	ErrGitRepositoryInvalid      = errors.New("GitHub repository is invalid")
	ErrGitRefInvalid             = errors.New("git ref is invalid")
	ErrGitPathInvalid            = errors.New("git build path is invalid")
	ErrDockerfileNotFound        = errors.New("Dockerfile not found in the configured build context")
)
