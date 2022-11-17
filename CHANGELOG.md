# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initialized Kubebuilder project.
- Added VPC controller, reconciler and AWS client.
- Added Helm chart with Deployment, ServiceAccount, Secret.
- Added ClusterRoles and ClusterRoleBindings.
- Added PodSecurityPolicy.
- Added NetworkPolicy.

### Changed

- Renamed project name in the template.
- Renamed `Makefile` to `Makefile.kubebuilder.mk`.

[Unreleased]: https://github.com/giantswarm/aws-vpc-operator/tree/main
