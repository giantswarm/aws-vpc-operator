# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.2] - 2026-01-30

### Changed

- Build with up-to-date pipelines.
- Migrate to App Build Suite (ABS).

## [0.8.1] - 2024-08-14

### Fixed

- Disable logger development mode to avoid panicking

## [0.8.0] - 2024-07-24

### Changed

- Update Go dependencies for CAPI/CAPA/controller-runtime

### Fixed

- Remove unsetting of subnet ID since `AWSCluster` CRD had a breaking change and requires the field

## [0.7.0] - 2024-06-27

### Changed

- Update imports to fetch CAPA CRs version `v1beta2`.

## [0.6.0] - 2024-02-19

### Added
- Add `global.podSecurityStandards.enforced` value for PSS migration.

### Changed

- Configure `gsoci.azurecr.io` as the default container image registry.

## [0.5.0] - 2023-07-20

### Added

- Add necessary values for PSS policy warnings.

### Changed

-  migrate from `secretmanager` VPC endpoint to `s3` endpoint for Flatcar.

## [0.4.0] - 2023-03-15

### Changed

- Don't unpause the cluster, because other controller will do it.

## [0.3.1] - 2023-02-27

### Added

- Add the use of the runtime/default seccomp profile. Allow required volume types in PSP so that pods can still be admitted.

### Changed

- Don't create VPC Endpoints if VPC Endpoint annotation is set to `UserManaged`
- Do not generate a name if `Name` tag already given so that we can never confuse the values
- Only wait for CAPA deletion of load balancer and security groups if its finalizer is still there

## [0.3.0] - 2023-01-30

### Changed

- Don't overwrite the `Name` tag specified in `AWSCluster` when creating subnets.

## [0.2.2] - 2023-01-13

### Fixed

- Added missing `tag` prefix to filters to ensure its using the AWS tags for filtering subnets

## [0.2.1] - 2023-01-13

### Fixed

- Ensure the cluster role is assumed when getting endpoints subnets

## [0.2.0] - 2023-01-13

### Fixed

- Allow creation of VPC Endpoints when there are multiple subnets in the same AZ

### Added

- Support for the `subnet.giantswarm.io/endpoints: true` AWS Tag on subnets to control which subnets are used for the VPC Endpoints

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

[Unreleased]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.8.2...HEAD
[0.8.2]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.8.1...v0.8.2
[0.8.1]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/giantswarm/aws-vpc-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/aws-vpc-operator/releases/tag/v0.1.0
