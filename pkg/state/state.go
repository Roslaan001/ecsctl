// Package state manages ecsctl remote state stored in S3.
// Each context gets its own isolated state and lock file under a key prefix:
//
//	my-bucket/
//	├── prod/state.json
//	├── prod/state.lock
//	├── staging/state.json
//	└── staging/state.lock
//
// Locking uses a HeadObject check followed by PutObject.
package state

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/user"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// ResourceType identifies the kind of ECS resource tracked in state.
type ResourceType string

const (
	ResourceTypeCluster ResourceType = "cluster"
	ResourceTypeService ResourceType = "service"
)

// Resource is a single ECS resource tracked in state.
type Resource struct {
	Type      ResourceType      `json:"type"`
	Name      string            `json:"name"`
	ARN       string            `json:"arn"`
	Region    string            `json:"region"`
	Cluster   string            `json:"cluster,omitempty"` // services only
	CreatedBy string            `json:"createdBy"`
	CreatedAt time.Time         `json:"createdAt"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// State is the top-level state document stored in S3.
type State struct {
	Version   string     `json:"version"`
	UpdatedAt time.Time  `json:"updatedAt"`
	Resources []Resource `json:"resources"`
}

// Backend reads and writes state to a specific key prefix inside an S3 bucket.
type Backend struct {
	s3       *s3.Client
	bucket   string
	stateKey string // e.g. "prod/state.json"
	lockKey  string // e.g. "prod/state.lock"
	kmsKeyID string // optional KMS key ID for state encryption
}

// NewBackend creates a Backend for the given bucket and key prefix.
// keyPrefix should match the context name or a custom path, e.g. "prod", "team-a/eu-west-2".
// State lives at <keyPrefix>/state.json and lock at <keyPrefix>/state.lock.
func NewBackend(ctx context.Context, bucket, keyPrefix, region, profile, kmsKeyID string) (*Backend, error) {
	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	if keyPrefix == "" {
		keyPrefix = "default"
	}

	return &Backend{
		s3:       s3.NewFromConfig(cfg),
		bucket:   bucket,
		stateKey: keyPrefix + "/state.json",
		lockKey:  keyPrefix + "/state.lock",
		kmsKeyID: kmsKeyID,
	}, nil
}

// Load reads the current state from S3. Returns an empty State if none exists yet.
func (b *Backend) Load(ctx context.Context) (*State, error) {
	out, err := b.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.stateKey),
	})
	if err != nil {
		var nsk *s3types.NoSuchKey
		if errors.As(err, &nsk) {
			return &State{Version: "1", Resources: []Resource{}}, nil
		}
		return nil, fmt.Errorf("reading state from s3://%s/%s: %w", b.bucket, b.stateKey, err)
	}
	defer func() { _ = out.Body.Close() }()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("reading state body: %w", err)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing state JSON: %w", err)
	}
	return &st, nil
}

// Save writes the state to S3 under the configured key prefix.
func (b *Backend) Save(ctx context.Context, st *State) error {
	st.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(b.stateKey),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/json"),
	}
	if b.kmsKeyID != "" {
		input.ServerSideEncryption = s3types.ServerSideEncryptionAwsKms
		input.SSEKMSKeyId = aws.String(b.kmsKeyID)
	}

	_, err = b.s3.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("writing state to s3://%s/%s: %w", b.bucket, b.stateKey, err)
	}
	return nil
}

// Lock acquires a distributed lock by writing a lock file conditional on its absence.
func (b *Backend) Lock(ctx context.Context) error {
	caller := currentUser()
	body := []byte(fmt.Sprintf(`{"lockedBy":%q,"lockedAt":%q}`, caller, time.Now().UTC().Format(time.RFC3339)))

	input := &s3.PutObjectInput{
		Bucket:      aws.String(b.bucket),
		Key:         aws.String(b.lockKey),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
		IfNoneMatch: aws.String("*"),
	}
	if b.kmsKeyID != "" {
		input.ServerSideEncryption = s3types.ServerSideEncryptionAwsKms
		input.SSEKMSKeyId = aws.String(b.kmsKeyID)
	}

	_, err := b.s3.PutObject(ctx, input)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "PreconditionFailed" {
			info, _ := b.lockInfo(ctx)
			return fmt.Errorf("state is locked by %s — wait or delete s3://%s/%s to force-unlock", info, b.bucket, b.lockKey)
		}
		return fmt.Errorf("acquiring lock: %w", err)
	}
	return nil
}

// Unlock releases the lock by deleting the lock object.
func (b *Backend) Unlock(ctx context.Context) error {
	_, err := b.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.lockKey),
	})
	return err
}

// lockInfo reads who holds the lock for a better error message.
func (b *Backend) lockInfo(ctx context.Context) (string, error) {
	out, err := b.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.lockKey),
	})
	if err != nil {
		return "unknown", nil
	}
	defer func() { _ = out.Body.Close() }()
	data, _ := io.ReadAll(out.Body)

	var info struct {
		LockedBy string `json:"lockedBy"`
		LockedAt string `json:"lockedAt"`
	}
	_ = json.Unmarshal(data, &info)
	if info.LockedBy == "" {
		return "unknown", nil
	}
	return fmt.Sprintf("%s (since %s)", info.LockedBy, info.LockedAt), nil
}

// currentUser returns the OS username for lock metadata.
func currentUser() string {
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return u.Username
}

// --- State helper methods ---

// AddResource adds or replaces a resource (upsert by type+name+cluster).
func (st *State) AddResource(r Resource) {
	for i, existing := range st.Resources {
		if existing.Type == r.Type && existing.Name == r.Name && existing.Cluster == r.Cluster {
			st.Resources[i] = r
			return
		}
	}
	st.Resources = append(st.Resources, r)
}

// RemoveResource removes a resource by type, name, and cluster.
func (st *State) RemoveResource(resourceType ResourceType, name, cluster string) {
	filtered := st.Resources[:0]
	for _, r := range st.Resources {
		if r.Type == resourceType && r.Name == name && r.Cluster == cluster {
			continue
		}
		filtered = append(filtered, r)
	}
	st.Resources = filtered
}

// RemoveClusterAndServices removes the cluster itself and any services deployed to it from the state.
func (st *State) RemoveClusterAndServices(clusterName string) {
	filtered := st.Resources[:0]
	for _, r := range st.Resources {
		if r.Type == ResourceTypeCluster && r.Name == clusterName {
			continue
		}
		if r.Type == ResourceTypeService && r.Cluster == clusterName {
			continue
		}
		filtered = append(filtered, r)
	}
	st.Resources = filtered
}

// FindClusters returns all cluster resources in state.
func (st *State) FindClusters() []Resource {
	var result []Resource
	for _, r := range st.Resources {
		if r.Type == ResourceTypeCluster {
			result = append(result, r)
		}
	}
	return result
}

// FindServices returns all service resources, optionally filtered by cluster.
func (st *State) FindServices(cluster string) []Resource {
	var result []Resource
	for _, r := range st.Resources {
		if r.Type == ResourceTypeService {
			if cluster == "" || r.Cluster == cluster {
				result = append(result, r)
			}
		}
	}
	return result
}
