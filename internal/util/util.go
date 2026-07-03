package util

import "strings"

// ShortARN returns the last segment of an ARN for display purposes.
// e.g. "arn:aws:ecs:us-east-1:123456789:cluster/my-cluster" → "my-cluster"
func ShortARN(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return arn
}

// Ptr returns a pointer to the given value. Convenience helper for SDK calls.
func Ptr[T any](v T) *T {
	return &v
}
