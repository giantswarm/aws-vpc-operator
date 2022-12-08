package tags

import (
	capa "sigs.k8s.io/cluster-api-provider-aws/api/v1beta1"
)

const (
	NameAWSProviderPrefix = "github.com/giantswarm/aws-vpc-operator/"
	NameAWSRole           = NameAWSProviderPrefix + "role"
)

// BuildParams is used to build tags around an aws resource.
//
// Copied from sigs.k8s.io/cluster-api-provider-aws.
type BuildParams struct {
	// ClusterName is the cluster associated with the resource.
	ClusterName string

	// ResourceID is the unique identifier of the resource to be tagged.
	ResourceID string

	// Name is the name of the resource, it's applied as the tag "Name" on AWS.
	Name string

	// Role is the role associated to the resource.
	Role string

	// Any additional tags to be added to the resource.
	Additional map[string]string
}

// Build builds tags including the cluster tag and returns them in map form.
//
// Copied from sigs.k8s.io/cluster-api-provider-aws.
func (p BuildParams) Build() map[string]string {
	tags := make(map[string]string)
	for k, v := range p.Additional {
		tags[k] = v
	}

	if p.Role != "" {
		tags[NameAWSRole] = p.Role
	} else {
		tags[NameAWSRole] = capa.CommonRoleTagValue
	}

	if p.Name != "" {
		tags["Name"] = p.Name
	}

	tags[NameAWSProviderPrefix+p.ClusterName] = "owned"

	return tags
}
