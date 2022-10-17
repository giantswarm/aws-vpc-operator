package tags

import (
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// BuildParamsToTagSpecification builds a TagSpecification for the specified resource type.
func BuildParamsToTagSpecification(ec2ResourceType ec2Types.ResourceType, tags map[string]string) ec2Types.TagSpecification {
	tagSpec := ec2Types.TagSpecification{
		ResourceType: ec2ResourceType,
	}

	// For testing, we need sorted keys
	sortedKeys := make([]string, 0, len(tags))
	for k := range tags {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		tagSpec.Tags = append(tagSpec.Tags, ec2Types.Tag{
			Key:   aws.String(key),
			Value: aws.String(tags[key]),
		})
	}

	return tagSpec
}

//// CreateTagSpecification converts map[string]string to EC2 tags.
//// Deprecated: use BuildParamsToTagSpecification
//func CreateTagSpecification(resourceName string, resourceType ec2Types.ResourceType, src map[string]string) ec2Types.TagSpecification {
//	const temporaryResourceID = "temporary-resource-id"
//	const role = "common"
//	tags := make([]ec2Types.Tag, len(src))
//
//	for key, value := range src {
//		tag := ec2Types.Tag{
//			Key:   aws.String(key),
//			Value: aws.String(value),
//		}
//		tags = append(tags, tag)
//	}
//
//	// Set more tags
//	moreTags := []ec2Types.Tag{
//		// Name tag
//		{
//			Key:   aws.String("Name"),
//			Value: aws.String(resourceName),
//		},
//		// Role tag
//		{
//			Key:   aws.String(capa.NameAWSClusterAPIRole),
//			Value: aws.String(role),
//		},
//	}
//	tags = append(tags, moreTags...)
//
//	tagSpec := ec2Types.TagSpecification{
//		ResourceType: resourceType,
//		Tags:         tags,
//	}
//
//	return tagSpec
//}
