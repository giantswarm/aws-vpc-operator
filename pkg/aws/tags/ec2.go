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

// ToMap converts EC2 tags to map[string]string.
func ToMap(src []ec2Types.Tag) map[string]string {
	tags := make(map[string]string, len(src))

	for _, t := range src {
		tags[*t.Key] = *t.Value
	}

	return tags
}
