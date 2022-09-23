<!--
Not all PRs will require all tests to be carried out. Delete where appropriate.
-->

<!--
MODIFY THIS AFTER your new app repo is in https://github.com/giantswarm/github
@team-halo-engineers will be automatically requested for review once
this PR has been submitted. (But not for drafts)
-->

This PR:

- adds/changes/removes etc

### Testing

Description on how aws-vpc-operator can be tested.

- [ ] fresh install works
- [ ] upgrade from previous version works
- [ ] aws-vpc-operator manages only VPCs that are not managed by CAPA

#### Other testing

Description of features to additionally test for aws-vpc-operator installations.

- [ ] check reconciliation of existing resources after upgrading
- [ ] AWS VPC works after aws-vpc-operator upgrade

<!--
Changelog must always be updated.
-->

### Checklist

- [ ] Update changelog in CHANGELOG.md.
- [ ] Make sure `values.yaml` and `values.schema.json` are valid.
