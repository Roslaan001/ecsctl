package aws

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	autoscalingTypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecsTypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/smithy-go"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
)

const (
	ecsOptimizedAL2023AMI = "/aws/service/ecs/optimized-ami/amazon-linux-2023/recommended/image_id"
	ecsEC2InstancePolicy  = "arn:aws:iam::aws:policy/service-role/AmazonEC2ContainerServiceforEC2Role"
)

type ec2ClusterNames struct {
	capacityProvider string
	launchTemplate   string
	autoScalingGroup string
	role             string
	profile          string
}

// CreateEC2Cluster creates an ECS cluster backed by a managed EC2 Auto Scaling
// group. It uses public subnets in the account's default VPC and an ECS-optimized
// Amazon Linux 2023 AMI. The default capacity provider strategy targets EC2.
func (c *Client) CreateEC2Cluster(ctx context.Context, cfg *ecscfg.ClusterConfig, instanceType string, instanceCount int32) (runErr error) {
	if c.ec2 == nil || c.awsScaling == nil || c.iam == nil || c.ssm == nil {
		return fmt.Errorf("EC2 cluster provisioning is unavailable on this AWS client")
	}
	if instanceType == "" {
		instanceType = "t3.small"
	}
	if instanceCount < 1 {
		instanceCount = 1
	}

	names := namesForEC2Cluster(cfg.Name)
	vpcID, subnetIDs, securityGroupID, err := c.defaultVPCNetwork(ctx)
	if err != nil {
		return err
	}
	ami, err := c.ecsOptimizedAMI(ctx)
	if err != nil {
		return err
	}

	clusterCfg := *cfg
	clusterCfg.CapacityProviders = []string{"FARGATE", "FARGATE_SPOT"}
	clusterCfg.DefaultCapacityProviderStrategy = nil
	if err := c.CreateCluster(ctx, &clusterCfg); err != nil {
		return err
	}
	clusterCreated := true
	launchTemplateCreated, groupCreated, roleCreated, profileCreated, providerCreated := false, false, false, false, false
	defer func() {
		if runErr == nil {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if providerCreated || groupCreated || launchTemplateCreated || profileCreated || roleCreated {
			if cleanupErr := c.deleteEC2ClusterResources(cleanupCtx, cfg.Name, true); cleanupErr != nil {
				runErr = fmt.Errorf("%w; EC2 rollback was incomplete: %v", runErr, cleanupErr)
				return
			}
		}
		if clusterCreated {
			if _, cleanupErr := c.ecs.DeleteCluster(cleanupCtx, &ecs.DeleteClusterInput{Cluster: awssdk.String(cfg.Name)}); cleanupErr != nil {
				runErr = fmt.Errorf("%w; deleting the partially created ECS cluster failed: %v", runErr, cleanupErr)
			}
		}
	}()

	_, err = c.iam.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 awssdk.String(names.role),
		AssumeRolePolicyDocument: awssdk.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ec2.amazonaws.com"},"Action":"sts:AssumeRole"}]}`),
		Description:              awssdk.String("ECS container instances managed by ecsctl for cluster " + cfg.Name),
		Tags:                     []iamTypes.Tag{{Key: awssdk.String("ecsctl:cluster"), Value: awssdk.String(cfg.Name)}},
	})
	if err != nil {
		return fmt.Errorf("creating ECS container-instance role: %w", err)
	}
	roleCreated = true
	if _, err := c.iam.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{RoleName: awssdk.String(names.role), PolicyArn: awssdk.String(ecsEC2InstancePolicy)}); err != nil {
		return fmt.Errorf("attaching ECS instance policy: %w", err)
	}
	if _, err := c.iam.CreateInstanceProfile(ctx, &iam.CreateInstanceProfileInput{
		InstanceProfileName: awssdk.String(names.profile),
		Tags:                []iamTypes.Tag{{Key: awssdk.String("ecsctl:cluster"), Value: awssdk.String(cfg.Name)}},
	}); err != nil {
		return fmt.Errorf("creating EC2 instance profile: %w", err)
	}
	profileCreated = true
	if _, err := c.iam.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{InstanceProfileName: awssdk.String(names.profile), RoleName: awssdk.String(names.role)}); err != nil {
		return fmt.Errorf("adding ECS role to instance profile: %w", err)
	}

	userData := fmt.Sprintf("#!/bin/bash\necho ECS_CLUSTER=%q >> /etc/ecs/ecs.config\necho ECS_LOGLEVEL=info >> /etc/ecs/ecs.config\n", cfg.Name)
	ltOut, err := c.ec2.CreateLaunchTemplate(ctx, &ec2.CreateLaunchTemplateInput{
		LaunchTemplateName: awssdk.String(names.launchTemplate),
		VersionDescription: awssdk.String("ecsctl EC2 cluster defaults"),
		LaunchTemplateData: &ec2Types.RequestLaunchTemplateData{
			ImageId:      awssdk.String(ami),
			InstanceType: ec2Types.InstanceType(instanceType),
			IamInstanceProfile: &ec2Types.LaunchTemplateIamInstanceProfileSpecificationRequest{
				Name: awssdk.String(names.profile),
			},
			UserData: awssdk.String(base64.StdEncoding.EncodeToString([]byte(userData))),
			NetworkInterfaces: []ec2Types.LaunchTemplateInstanceNetworkInterfaceSpecificationRequest{{
				DeviceIndex:              awssdk.Int32(0),
				Groups:                   []string{securityGroupID},
				AssociatePublicIpAddress: awssdk.Bool(true),
			}},
			MetadataOptions: &ec2Types.LaunchTemplateInstanceMetadataOptionsRequest{HttpTokens: ec2Types.LaunchTemplateHttpTokensStateRequired},
		},
		TagSpecifications: []ec2Types.TagSpecification{{
			ResourceType: ec2Types.ResourceTypeLaunchTemplate,
			Tags:         []ec2Types.Tag{{Key: awssdk.String("ecsctl:cluster"), Value: awssdk.String(cfg.Name)}},
		}},
	})
	if err != nil {
		return fmt.Errorf("creating EC2 launch template: %w", err)
	}
	launchTemplateCreated = true

	maximum := instanceCount * 3
	if maximum < 3 {
		maximum = 3
	}
	if _, err := c.awsScaling.CreateAutoScalingGroup(ctx, &autoscaling.CreateAutoScalingGroupInput{
		AutoScalingGroupName: awssdk.String(names.autoScalingGroup),
		MinSize:              awssdk.Int32(0),
		MaxSize:              awssdk.Int32(maximum),
		DesiredCapacity:      awssdk.Int32(instanceCount),
		VPCZoneIdentifier:    awssdk.String(strings.Join(subnetIDs, ",")),
		LaunchTemplate: &autoscalingTypes.LaunchTemplateSpecification{
			LaunchTemplateId: awssdk.String(awssdk.ToString(ltOut.LaunchTemplate.LaunchTemplateId)),
			Version:          awssdk.String("$Latest"),
		},
		HealthCheckType:                  awssdk.String("EC2"),
		HealthCheckGracePeriod:           awssdk.Int32(300),
		NewInstancesProtectedFromScaleIn: awssdk.Bool(false),
		Tags: []autoscalingTypes.Tag{{
			Key: awssdk.String("ecsctl:cluster"), Value: awssdk.String(cfg.Name),
			ResourceId: awssdk.String(names.autoScalingGroup), ResourceType: awssdk.String("auto-scaling-group"), PropagateAtLaunch: awssdk.Bool(true),
		}},
	}); err != nil {
		return fmt.Errorf("creating ECS EC2 Auto Scaling group in VPC %s: %w", vpcID, err)
	}
	groupCreated = true

	groupOut, err := c.awsScaling.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []string{names.autoScalingGroup}})
	if err != nil || len(groupOut.AutoScalingGroups) == 0 || awssdk.ToString(groupOut.AutoScalingGroups[0].AutoScalingGroupARN) == "" {
		if err != nil {
			return fmt.Errorf("describing ECS EC2 Auto Scaling group: %w", err)
		}
		return fmt.Errorf("ECS EC2 Auto Scaling group %q has no ARN", names.autoScalingGroup)
	}
	_, err = c.ecs.CreateCapacityProvider(ctx, &ecs.CreateCapacityProviderInput{
		Name: awssdk.String(names.capacityProvider),
		AutoScalingGroupProvider: &ecsTypes.AutoScalingGroupProvider{
			AutoScalingGroupArn:          groupOut.AutoScalingGroups[0].AutoScalingGroupARN,
			ManagedDraining:              ecsTypes.ManagedDrainingEnabled,
			ManagedTerminationProtection: ecsTypes.ManagedTerminationProtectionDisabled,
			ManagedScaling: &ecsTypes.ManagedScaling{
				Status: ecsTypes.ManagedScalingStatusEnabled, TargetCapacity: awssdk.Int32(100),
				MinimumScalingStepSize: awssdk.Int32(1), MaximumScalingStepSize: awssdk.Int32(3), InstanceWarmupPeriod: awssdk.Int32(300),
			},
		},
		Tags: toECSTags(cfg.Tags),
	})
	if err != nil {
		return fmt.Errorf("creating ECS EC2 capacity provider: %w", err)
	}
	providerCreated = true
	if err := c.waitForCapacityProvider(ctx, names.capacityProvider); err != nil {
		return err
	}

	providerNames := []string{"FARGATE", "FARGATE_SPOT", names.capacityProvider}
	_, err = c.ecs.PutClusterCapacityProviders(ctx, &ecs.PutClusterCapacityProvidersInput{
		Cluster: awssdk.String(cfg.Name), CapacityProviders: providerNames,
		DefaultCapacityProviderStrategy: []ecsTypes.CapacityProviderStrategyItem{{CapacityProvider: awssdk.String(names.capacityProvider), Weight: 1}},
	})
	if err != nil {
		return fmt.Errorf("setting EC2 as the cluster's default capacity provider: %w", err)
	}

	if err := c.waitForEC2ContainerInstances(ctx, cfg.Name, instanceCount); err != nil {
		return err
	}
	return nil
}

func (c *Client) waitForCapacityProvider(ctx context.Context, providerName string) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(5 * time.Minute)
	defer deadline.Stop()
	for {
		out, err := c.ecs.DescribeCapacityProviders(ctx, &ecs.DescribeCapacityProvidersInput{CapacityProviders: []string{providerName}})
		if err != nil {
			return fmt.Errorf("waiting for EC2 capacity provider: %w", err)
		}
		if len(out.CapacityProviders) > 0 && out.CapacityProviders[0].Status == ecsTypes.CapacityProviderStatusActive {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for EC2 capacity provider %q to become active", providerName)
		case <-ticker.C:
		}
	}
}

func (c *Client) defaultVPCNetwork(ctx context.Context) (string, []string, string, error) {
	vpcs, err := c.ec2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{Filters: []ec2Types.Filter{{Name: awssdk.String("isDefault"), Values: []string{"true"}}}})
	if err != nil {
		return "", nil, "", fmt.Errorf("finding the account's default VPC: %w", err)
	}
	if len(vpcs.Vpcs) == 0 || awssdk.ToString(vpcs.Vpcs[0].VpcId) == "" {
		return "", nil, "", fmt.Errorf("EC2 cluster creation needs a default VPC; create a default VPC in this AWS region or use Fargate")
	}
	vpcID := awssdk.ToString(vpcs.Vpcs[0].VpcId)
	subnets, err := c.ec2.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{Filters: []ec2Types.Filter{
		{Name: awssdk.String("vpc-id"), Values: []string{vpcID}},
		{Name: awssdk.String("state"), Values: []string{"available"}},
	}})
	if err != nil {
		return "", nil, "", fmt.Errorf("finding public subnets in default VPC %s: %w", vpcID, err)
	}
	subnetIDs := make([]string, 0, len(subnets.Subnets))
	for _, subnet := range subnets.Subnets {
		if subnet.MapPublicIpOnLaunch != nil && *subnet.MapPublicIpOnLaunch && awssdk.ToString(subnet.SubnetId) != "" {
			subnetIDs = append(subnetIDs, awssdk.ToString(subnet.SubnetId))
		}
	}
	if len(subnetIDs) == 0 {
		return "", nil, "", fmt.Errorf("default VPC %s has no public subnets; enable public IPv4 assignment on a subnet or use Fargate", vpcID)
	}
	sort.Strings(subnetIDs)
	groups, err := c.ec2.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{Filters: []ec2Types.Filter{
		{Name: awssdk.String("vpc-id"), Values: []string{vpcID}},
		{Name: awssdk.String("group-name"), Values: []string{"default"}},
	}})
	if err != nil {
		return "", nil, "", fmt.Errorf("finding the default security group in VPC %s: %w", vpcID, err)
	}
	if len(groups.SecurityGroups) == 0 || awssdk.ToString(groups.SecurityGroups[0].GroupId) == "" {
		return "", nil, "", fmt.Errorf("default VPC %s has no default security group", vpcID)
	}
	return vpcID, subnetIDs, awssdk.ToString(groups.SecurityGroups[0].GroupId), nil
}

func (c *Client) ecsOptimizedAMI(ctx context.Context) (string, error) {
	out, err := c.ssm.GetParameter(ctx, &ssm.GetParameterInput{Name: awssdk.String(ecsOptimizedAL2023AMI)})
	if err != nil {
		return "", fmt.Errorf("resolving the recommended ECS-optimized Amazon Linux 2023 AMI: %w", err)
	}
	if out.Parameter == nil || awssdk.ToString(out.Parameter.Value) == "" {
		return "", fmt.Errorf("systems manager returned no AMI for %s", ecsOptimizedAL2023AMI)
	}
	return awssdk.ToString(out.Parameter.Value), nil
}

func (c *Client) waitForEC2ContainerInstances(ctx context.Context, clusterName string, count int32) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	deadline := time.NewTimer(10 * time.Minute)
	defer deadline.Stop()
	for {
		out, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{Clusters: []string{clusterName}, Include: []ecsTypes.ClusterField{ecsTypes.ClusterFieldStatistics}})
		if err != nil {
			return fmt.Errorf("waiting for EC2 container instances to register: %w", err)
		}
		if len(out.Clusters) > 0 && int32(out.Clusters[0].RegisteredContainerInstancesCount) >= count {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for EC2 instances to register in cluster %q; check the default VPC's internet route and the instance role", clusterName)
		case <-ticker.C:
		}
	}
}

func namesForEC2Cluster(clusterName string) ec2ClusterNames {
	var safe strings.Builder
	for _, r := range strings.ToLower(clusterName) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			safe.WriteRune(r)
		} else {
			safe.WriteByte('-')
		}
	}
	base := strings.Trim(safe.String(), "-")
	if len(base) > 30 {
		base = base[:30]
	}
	hash := sha256.Sum256([]byte(clusterName))
	prefix := fmt.Sprintf("ecsctl-%s-%s", base, hex.EncodeToString(hash[:4]))
	return ec2ClusterNames{
		capacityProvider: prefix + "-ec2",
		launchTemplate:   prefix + "-lt",
		autoScalingGroup: prefix + "-asg",
		role:             prefix + "-instance-role",
		profile:          prefix + "-instance-profile",
	}
}

func (c *Client) detachAndDeleteCapacityProvider(ctx context.Context, clusterName, providerName string) error {
	providerOut, err := c.ecs.DescribeCapacityProviders(ctx, &ecs.DescribeCapacityProvidersInput{CapacityProviders: []string{providerName}})
	if err != nil {
		return fmt.Errorf("checking EC2 capacity provider before deletion: %w", err)
	}
	if len(providerOut.CapacityProviders) == 0 || providerOut.CapacityProviders[0].Status == ecsTypes.CapacityProviderStatusInactive {
		return nil
	}
	clusterOut, err := c.ecs.DescribeClusters(ctx, &ecs.DescribeClustersInput{Clusters: []string{clusterName}})
	if err != nil {
		return fmt.Errorf("describing cluster before removing EC2 capacity: %w", err)
	}
	if len(clusterOut.Clusters) == 0 {
		return nil
	}
	cluster := clusterOut.Clusters[0]
	providers := make([]string, 0, len(cluster.CapacityProviders))
	found := false
	for _, provider := range cluster.CapacityProviders {
		if provider == providerName {
			found = true
			continue
		}
		providers = append(providers, provider)
	}
	if found {
		strategy := make([]ecsTypes.CapacityProviderStrategyItem, 0, len(cluster.DefaultCapacityProviderStrategy))
		for _, item := range cluster.DefaultCapacityProviderStrategy {
			if awssdk.ToString(item.CapacityProvider) != providerName {
				strategy = append(strategy, item)
			}
		}
		if len(strategy) == 0 {
			if !containsString(providers, "FARGATE") {
				providers = append(providers, "FARGATE")
			}
			strategy = []ecsTypes.CapacityProviderStrategyItem{{CapacityProvider: awssdk.String("FARGATE"), Weight: 1}}
		}
		if _, err := c.ecs.PutClusterCapacityProviders(ctx, &ecs.PutClusterCapacityProvidersInput{Cluster: awssdk.String(clusterName), CapacityProviders: providers, DefaultCapacityProviderStrategy: strategy}); err != nil {
			return fmt.Errorf("disassociating EC2 capacity provider from cluster: %w", err)
		}
	}
	_, err = c.ecs.DeleteCapacityProvider(ctx, &ecs.DeleteCapacityProviderInput{CapacityProvider: awssdk.String(providerName)})
	if err != nil {
		return fmt.Errorf("deleting EC2 capacity provider: %w", err)
	}
	for {
		out, err := c.ecs.DescribeCapacityProviders(ctx, &ecs.DescribeCapacityProvidersInput{CapacityProviders: []string{providerName}})
		if err != nil {
			return fmt.Errorf("waiting for EC2 capacity provider deletion: %w", err)
		}
		if len(out.CapacityProviders) == 0 || out.CapacityProviders[0].Status == ecsTypes.CapacityProviderStatusInactive {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (c *Client) deleteEC2ClusterResources(ctx context.Context, clusterName string, force bool) error {
	if c.ec2 == nil || c.awsScaling == nil || c.iam == nil {
		return nil
	}
	if !force {
		providerOut, err := c.ecs.DescribeCapacityProviders(ctx, &ecs.DescribeCapacityProvidersInput{CapacityProviders: []string{namesForEC2Cluster(clusterName).capacityProvider}})
		if err != nil {
			return fmt.Errorf("checking for ecsctl-managed EC2 capacity: %w", err)
		}
		if len(providerOut.CapacityProviders) == 0 {
			return nil
		}
	}
	names := namesForEC2Cluster(clusterName)
	if err := c.detachAndDeleteCapacityProvider(ctx, clusterName, names.capacityProvider); err != nil {
		return err
	}
	groups, err := c.awsScaling.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []string{names.autoScalingGroup}})
	if err != nil {
		return fmt.Errorf("checking for ECS EC2 Auto Scaling group: %w", err)
	}
	if len(groups.AutoScalingGroups) > 0 {
		if _, err := c.awsScaling.DeleteAutoScalingGroup(ctx, &autoscaling.DeleteAutoScalingGroupInput{AutoScalingGroupName: awssdk.String(names.autoScalingGroup), ForceDelete: awssdk.Bool(true)}); err != nil {
			return fmt.Errorf("deleting ECS EC2 Auto Scaling group: %w", err)
		}
		for {
			groups, err = c.awsScaling.DescribeAutoScalingGroups(ctx, &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []string{names.autoScalingGroup}})
			if err != nil {
				return fmt.Errorf("waiting for ECS EC2 Auto Scaling group deletion: %w", err)
			}
			if len(groups.AutoScalingGroups) == 0 {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
	}
	launchTemplates, err := c.ec2.DescribeLaunchTemplates(ctx, &ec2.DescribeLaunchTemplatesInput{LaunchTemplateNames: []string{names.launchTemplate}})
	if err != nil && !isLaunchTemplateMissing(err) {
		return fmt.Errorf("checking for ECS EC2 launch template: %w", err)
	}
	if err == nil && len(launchTemplates.LaunchTemplates) > 0 {
		if _, err := c.ec2.DeleteLaunchTemplate(ctx, &ec2.DeleteLaunchTemplateInput{LaunchTemplateName: awssdk.String(names.launchTemplate)}); err != nil {
			return fmt.Errorf("deleting ECS EC2 launch template: %w", err)
		}
	}
	return c.deleteEC2InstanceRole(ctx, names)
}

func (c *Client) deleteEC2InstanceRole(ctx context.Context, names ec2ClusterNames) error {
	profileOut, err := c.iam.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{InstanceProfileName: awssdk.String(names.profile)})
	if err != nil && !isIAMMissing(err) {
		return fmt.Errorf("checking ECS EC2 instance profile: %w", err)
	}
	if err == nil && profileOut.InstanceProfile != nil {
		for _, role := range profileOut.InstanceProfile.Roles {
			if _, err := c.iam.RemoveRoleFromInstanceProfile(ctx, &iam.RemoveRoleFromInstanceProfileInput{InstanceProfileName: awssdk.String(names.profile), RoleName: role.RoleName}); err != nil && !isIAMMissing(err) {
				return fmt.Errorf("removing role from instance profile: %w", err)
			}
		}
		if _, err := c.iam.DeleteInstanceProfile(ctx, &iam.DeleteInstanceProfileInput{InstanceProfileName: awssdk.String(names.profile)}); err != nil && !isIAMMissing(err) {
			return fmt.Errorf("deleting instance profile: %w", err)
		}
	}
	_, err = c.iam.GetRole(ctx, &iam.GetRoleInput{RoleName: awssdk.String(names.role)})
	if isIAMMissing(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking ECS EC2 instance role: %w", err)
	}
	if _, err := c.iam.DetachRolePolicy(ctx, &iam.DetachRolePolicyInput{RoleName: awssdk.String(names.role), PolicyArn: awssdk.String(ecsEC2InstancePolicy)}); err != nil && !isIAMMissing(err) {
		return fmt.Errorf("detaching ECS instance policy: %w", err)
	}
	if _, err := c.iam.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: awssdk.String(names.role)}); err != nil && !isIAMMissing(err) {
		return fmt.Errorf("deleting ECS instance role: %w", err)
	}
	return nil
}

func isIAMMissing(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchEntity"
}

func isLaunchTemplateMissing(err error) bool {
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "InvalidLaunchTemplateName.NotFound"
}
