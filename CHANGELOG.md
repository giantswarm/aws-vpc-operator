# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] - 2022-12-08

## [0.1.1] - 2022-12-07

## [0.1.0] - 2022-11-23

### Added

- Initialized Kubebuilder project.
- Added VPC controller, reconciler and AWS client.
- Added Helm chart with Deployment, ServiceAccount, Secret.
- Added ClusterRoles and ClusterRoleBindings.
- Added PodSecurityPolicy.
- Added NetworkPolicy.
- Add `config.giantswarm.io/version: 1.x.x` annotation to `Chart.yaml`

### Changed

- Renamed project name in the template.
- Renamed `Makefile` to `Makefile.kubebuilder.mk`.

[Unreleased]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/aws-vpc-operator/releases/tag/v0.1.0
