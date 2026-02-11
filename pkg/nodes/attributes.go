package nodes

import (
	"strings"
)

// buildExtraTags builds additional tags from options, labels, and annotations.
func buildExtraTags(opts DiscoverOptions, workloadLabels map[string]string) []string {
	var tags []string

	// Add custom static tags
	tags = append(tags, opts.AddTags...)

	// Add labels as tags
	for _, labelKey := range opts.LabelsAsTags {
		if value, ok := workloadLabels[labelKey]; ok && value != "" {
			// Format: labelKey:value (sanitize the key for tag format)
			sanitizedKey := sanitizeTagKey(labelKey)
			tags = append(tags, sanitizedKey+":"+value)
		}
	}

	return tags
}

// buildExtraAttributes builds extra node attributes from labels and annotations.
func buildExtraAttributes(opts DiscoverOptions, workloadLabels, workloadAnnotations map[string]string) map[string]string {
	attrs := make(map[string]string)

	// Add label attributes
	for _, labelKey := range opts.LabelAttributes {
		if value, ok := workloadLabels[labelKey]; ok {
			attrName := "label_" + sanitizeAttributeKey(labelKey)
			attrs[attrName] = value
		}
	}

	// Add annotation attributes
	for _, annotationKey := range opts.AnnotationAttributes {
		if value, ok := workloadAnnotations[annotationKey]; ok {
			attrName := "annotation_" + sanitizeAttributeKey(annotationKey)
			attrs[attrName] = value
		}
	}

	return attrs
}

// sanitizeTagKey converts a Kubernetes label/annotation key to a tag-safe format.
// Example: "app.kubernetes.io/name" -> "app_kubernetes_io_name"
func sanitizeTagKey(key string) string {
	key = strings.ReplaceAll(key, ".", "_")
	key = strings.ReplaceAll(key, "/", "_")
	key = strings.ReplaceAll(key, "-", "_")
	return key
}

// sanitizeAttributeKey converts a Kubernetes label/annotation key to an attribute-safe format.
// Example: "app.kubernetes.io/name" -> "app_kubernetes_io_name"
func sanitizeAttributeKey(key string) string {
	key = strings.ReplaceAll(key, ".", "_")
	key = strings.ReplaceAll(key, "/", "_")
	key = strings.ReplaceAll(key, "-", "_")
	return key
}

// mergeTagStrings merges a base tags string with additional tags.
func mergeTagStrings(baseTags string, extraTags []string) string {
	if len(extraTags) == 0 {
		return baseTags
	}

	allTags := baseTags
	for _, tag := range extraTags {
		if allTags != "" {
			allTags += ","
		}
		allTags += tag
	}
	return allTags
}
