package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ecscfg "github.com/roslaan001/ecsctl/pkg/config"
)

const (
	defaultGatewayRoleName = "ecsInfrastructureRoleForGateways"
	ecsGatewayPolicy       = "arn:aws:iam::aws:policy/AmazonECSInfrastructureRoleForGatewayHostSharedALB"
	ecsGatewayTrustPolicy  = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ecs.amazonaws.com"},"Action":"sts:AssumeRole"}]}`
)

// gatewayTrustPolicy generates an IAM assume role trust policy for the ECS Gateway
// infrastructure role. When an account ID is available, it adds the aws:SourceAccount
// condition to prevent the confused deputy problem per AWS security best practices.
func gatewayTrustPolicy(accountID string) string {
	if accountID == "" {
		return ecsGatewayTrustPolicy
	}
	return fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"ecs.amazonaws.com"},"Action":"sts:AssumeRole","Condition":{"StringEquals":{"aws:SourceAccount":"%s"}}}]}`, accountID)
}

// EnsureGatewayInfrastructureRole ensures an IAM role for ECS Gateway exists and returns its ARN.
// If roleArnOrName is empty, it checks for "ecsInfrastructureRoleForGateways".
// If the role does not exist, it automatically creates it with the trusted ECS entity and
// attaches the AmazonECSInfrastructureRoleForGatewayHostSharedALB policy.
func (c *Client) EnsureGatewayInfrastructureRole(ctx context.Context, roleArnOrName string) (string, error) {
	if roleArnOrName != "" && strings.HasPrefix(roleArnOrName, "arn:aws:iam::") && !strings.HasSuffix(roleArnOrName, "/"+defaultGatewayRoleName) {
		return roleArnOrName, nil
	}
	if c.iam == nil {
		if roleArnOrName != "" {
			return roleArnOrName, nil
		}
		return "arn:aws:iam::123456789012:role/" + defaultGatewayRoleName, nil
	}

	roleName := roleArnOrName
	if roleName == "" {
		roleName = defaultGatewayRoleName
	} else if strings.HasPrefix(roleName, "arn:aws:iam::") {
		parts := strings.Split(roleName, "/")
		roleName = parts[len(parts)-1]
	}

	getOut, err := c.iam.GetRole(ctx, &iam.GetRoleInput{
		RoleName: aws.String(roleName),
	})
	if err == nil && getOut.Role != nil {
		_, _ = c.iam.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
			RoleName:  aws.String(roleName),
			PolicyArn: aws.String(ecsGatewayPolicy),
		})
		return aws.ToString(getOut.Role.Arn), nil
	}
	if err != nil && !isIAMMissing(err) {
		return "", fmt.Errorf("checking IAM role %q: %w", roleName, err)
	}

	fmt.Printf("Provisioning IAM role %q for ECS Gateway...\n", roleName)
	var accountID string
	if c.sts != nil {
		callerOut, err := c.sts.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
		if err == nil && callerOut.Account != nil {
			accountID = aws.ToString(callerOut.Account)
		}
	}

	createOut, err := c.iam.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(gatewayTrustPolicy(accountID)),
		Description:              aws.String("ECS infrastructure role for gateways managed by ecsctl"),
	})
	if err != nil {
		return "", fmt.Errorf("creating gateway infrastructure role %q: %w", roleName, err)
	}

	if _, err := c.iam.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		RoleName:  aws.String(roleName),
		PolicyArn: aws.String(ecsGatewayPolicy),
	}); err != nil {
		return "", fmt.Errorf("attaching gateway policy to role %q: %w", roleName, err)
	}

	return aws.ToString(createOut.Role.Arn), nil
}

// CreateGateway creates an ECS Gateway and associates it with any specified clusters.
func (c *Client) CreateGateway(ctx context.Context, cfg *ecscfg.GatewayConfig) (string, error) {
	if cfg.InfrastructureRoleARN == "" || strings.HasSuffix(cfg.InfrastructureRoleARN, "/"+defaultGatewayRoleName) || cfg.InfrastructureRoleARN == defaultGatewayRoleName {
		roleArn, err := c.EnsureGatewayInfrastructureRole(ctx, cfg.InfrastructureRoleARN)
		if err != nil {
			return "", fmt.Errorf("ensuring gateway infrastructure role: %w", err)
		}
		cfg.InfrastructureRoleARN = roleArn
	}

	input := &ecs.CreateGatewayInput{
		Name:                  aws.String(cfg.Name),
		InfrastructureRoleArn: aws.String(cfg.InfrastructureRoleARN),
		DeletionProtection:    cfg.DeletionProtection,
		Tags:                  toECSTags(cfg.Tags),
	}

	hostShared := types.HostSharedAlbConfiguration{}
	if cfg.Configuration != nil && cfg.Configuration.HostSharedAlb != nil {
		albCfg := cfg.Configuration.HostSharedAlb
		if albCfg.Scheme != "" {
			hostShared.Scheme = types.GatewayElbScheme(albCfg.Scheme)
		}
		if albCfg.MaxAlbCount > 0 {
			hostShared.MaxAlbCount = aws.Int32(albCfg.MaxAlbCount)
		}
		if albCfg.WebACLArn != "" {
			hostShared.WebACLArn = aws.String(albCfg.WebACLArn)
		}
		if albCfg.Network != nil {
			net := &types.GatewayElbNetworkConfiguration{
				Subnets: albCfg.Network.Subnets,
			}
			if albCfg.Network.InboundSecurityGroup != "" {
				net.InboundSecurityGroup = aws.String(albCfg.Network.InboundSecurityGroup)
			}
			if albCfg.Network.IPAddressType != "" {
				net.IpAddressType = types.GatewayElbIpAddressType(albCfg.Network.IPAddressType)
			}
			hostShared.NetworkConfiguration = net
		}
		if albCfg.AccessLogs != nil && albCfg.AccessLogs.S3BucketName != "" {
			hostShared.AccessLogs = &types.GatewayElbAccessLogConfiguration{
				Destination: &types.GatewayElbAccessLogDestinationMemberS3{
					Value: types.GatewayElbAccessLogS3Destination{
						Bucket: aws.String(albCfg.AccessLogs.S3BucketName),
						Prefix: optionalString(albCfg.AccessLogs.S3Prefix),
					},
				},
			}
		}
	}
	input.Configuration = &types.GatewayConfigurationMemberHostSharedAlb{
		Value: hostShared,
	}

	out, err := c.ecs.CreateGateway(ctx, input)
	if err != nil {
		return "", fmt.Errorf("creating gateway: %w", err)
	}
	if out.GatewayArn == nil {
		return "", fmt.Errorf("ECS returned empty gateway ARN in response")
	}
	gwArn := aws.ToString(out.GatewayArn)

	for _, cluster := range cfg.Clusters {
		if err := c.AssociateGatewayWithCluster(ctx, gwArn, cluster); err != nil {
			return gwArn, fmt.Errorf("gateway created (%s) but failed associating cluster %q: %w", gwArn, cluster, err)
		}
	}

	return gwArn, nil
}

// DescribeGateway retrieves full details of an ECS Gateway.
func (c *Client) DescribeGateway(ctx context.Context, nameOrArn string) (*types.Gateway, error) {
	out, err := c.ecs.DescribeGateway(ctx, &ecs.DescribeGatewayInput{
		Gateway: aws.String(nameOrArn),
	})
	if err != nil {
		return nil, fmt.Errorf("describing gateway: %w", err)
	}
	if out.Gateway == nil {
		return nil, fmt.Errorf("gateway %q not found", nameOrArn)
	}
	return out.Gateway, nil
}

// DeleteGateway deletes an ECS Gateway.
func (c *Client) DeleteGateway(ctx context.Context, nameOrArn string) error {
	_, err := c.ecs.DeleteGateway(ctx, &ecs.DeleteGatewayInput{
		Gateway: aws.String(nameOrArn),
	})
	if err != nil {
		return fmt.Errorf("deleting gateway: %w", err)
	}
	return nil
}

// ListGateways returns a summary of all ECS Gateways.
func (c *Client) ListGateways(ctx context.Context) ([]types.GatewaySummary, error) {
	var summaries []types.GatewaySummary
	var nextToken *string

	for {
		out, err := c.ecs.ListGateways(ctx, &ecs.ListGatewaysInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing gateways: %w", err)
		}
		summaries = append(summaries, out.Gateways...)
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return summaries, nil
}

// AssociateGatewayWithCluster associates a gateway with an ECS cluster.
func (c *Client) AssociateGatewayWithCluster(ctx context.Context, gateway, cluster string) error {
	_, err := c.ecs.AssociateGatewayWithCluster(ctx, &ecs.AssociateGatewayWithClusterInput{
		Gateway: aws.String(gateway),
		Cluster: aws.String(cluster),
	})
	if err != nil {
		return fmt.Errorf("associating gateway %q with cluster %q: %w", gateway, cluster, err)
	}
	return nil
}

// DisassociateGatewayFromCluster disassociates a gateway from an ECS cluster.
func (c *Client) DisassociateGatewayFromCluster(ctx context.Context, gateway, cluster string) error {
	_, err := c.ecs.DisassociateGatewayFromCluster(ctx, &ecs.DisassociateGatewayFromClusterInput{
		Gateway: aws.String(gateway),
		Cluster: aws.String(cluster),
	})
	if err != nil {
		return fmt.Errorf("disassociating gateway %q from cluster %q: %w", gateway, cluster, err)
	}
	return nil
}

// ListGatewayClusters returns the list of clusters associated with a gateway.
func (c *Client) ListGatewayClusters(ctx context.Context, gateway string) ([]string, error) {
	var clusters []string
	var nextToken *string

	for {
		out, err := c.ecs.ListGatewayClusterAssociations(ctx, &ecs.ListGatewayClusterAssociationsInput{
			Gateway:   aws.String(gateway),
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing gateway cluster associations: %w", err)
		}
		for _, assoc := range out.GatewayClusterAssociations {
			if assoc.ClusterArn != nil {
				clusters = append(clusters, aws.ToString(assoc.ClusterArn))
			}
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return clusters, nil
}

// DescribeGatewayResources retrieves the infrastructure resources managed by a gateway.
func (c *Client) DescribeGatewayResources(ctx context.Context, gateway string) (*types.GatewayResources, error) {
	out, err := c.ecs.DescribeGatewayResources(ctx, &ecs.DescribeGatewayResourcesInput{
		Gateway: aws.String(gateway),
	})
	if err != nil {
		return nil, fmt.Errorf("describing gateway resources: %w", err)
	}
	return out.GatewayResources, nil
}

// ListGatewayRoutes returns all routes for services attached to a gateway.
func (c *Client) ListGatewayRoutes(ctx context.Context, gateway string) ([]types.GatewayRoute, error) {
	var routes []types.GatewayRoute
	var nextToken *string

	for {
		out, err := c.ecs.ListGatewayRoutes(ctx, &ecs.ListGatewayRoutesInput{
			Gateway:   aws.String(gateway),
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing gateway routes: %w", err)
		}
		for _, rSummary := range out.GatewayRoutes {
			if rSummary.GatewayRouteArn != nil {
				rDetail, err := c.ecs.DescribeGatewayRoute(ctx, &ecs.DescribeGatewayRouteInput{
					GatewayRouteArn: rSummary.GatewayRouteArn,
				})
				if err == nil && rDetail.GatewayRoute != nil {
					routes = append(routes, *rDetail.GatewayRoute)
				}
			}
		}
		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return routes, nil
}

// PrintGatewayDetail outputs a formatted description of the gateway.
func (c *Client) PrintGatewayDetail(ctx context.Context, nameOrArn string, showResources bool) error {
	gw, err := c.DescribeGateway(ctx, nameOrArn)
	if err != nil {
		return err
	}

	fmt.Printf("Name:                  %s\n", aws.ToString(gw.Name))
	fmt.Printf("ARN:                   %s\n", aws.ToString(gw.GatewayArn))
	fmt.Printf("Status:                %s\n", gw.Status)
	if gw.StatusReason != nil {
		fmt.Printf("Status Reason:         %s\n", aws.ToString(gw.StatusReason))
	}
	fmt.Printf("Access Type:           %s\n", gw.AccessType)
	if gw.DeletionProtection != nil {
		fmt.Printf("Deletion Protection:   %t\n", *gw.DeletionProtection)
	}
	fmt.Printf("Infrastructure Role:   %s\n", aws.ToString(gw.InfrastructureRoleArn))

	if albMember, ok := gw.Configuration.(*types.DescribeGatewayConfigurationMemberHostSharedAlb); ok {
		alb := albMember.Value
		fmt.Printf("\nConfiguration (HostSharedAlb):\n")
		fmt.Printf("  Scheme:              %s\n", alb.Scheme)
		if alb.CurrentAlbCount != nil {
			max := int32(10)
			if alb.MaxAlbCount != nil {
				max = *alb.MaxAlbCount
			}
			fmt.Printf("  Managed ALBs:        %d / %d\n", *alb.CurrentAlbCount, max)
		}
		if alb.WebACLArn != nil {
			fmt.Printf("  Web ACL ARN:         %s\n", aws.ToString(alb.WebACLArn))
		}
		if alb.NetworkConfiguration != nil {
			if len(alb.NetworkConfiguration.Subnets) > 0 {
				fmt.Printf("  Subnets:             %s\n", strings.Join(alb.NetworkConfiguration.Subnets, ", "))
			}
			if alb.NetworkConfiguration.InboundSecurityGroup != nil {
				fmt.Printf("  Inbound SG:          %s\n", aws.ToString(alb.NetworkConfiguration.InboundSecurityGroup))
			}
			if alb.NetworkConfiguration.IpAddressType != "" {
				fmt.Printf("  IP Address Type:     %s\n", alb.NetworkConfiguration.IpAddressType)
			}
		}
	}

	// Associated Clusters
	clusters, err := c.ListGatewayClusters(ctx, nameOrArn)
	if err == nil && len(clusters) > 0 {
		fmt.Printf("\nAssociated Clusters:\n")
		for _, cl := range clusters {
			fmt.Printf("  - %s\n", cl)
		}
	}

	// Active Failures
	if len(gw.Failures) > 0 {
		fmt.Printf("\nFailures:\n")
		for _, f := range gw.Failures {
			fmt.Printf("  - Resource: %s\n", aws.ToString(f.Arn))
			for _, opErr := range f.Errors {
				fmt.Printf("    * [%s] %s: %s\n", aws.ToString(opErr.Code), aws.ToString(opErr.Operation), aws.ToString(opErr.Message))
			}
		}
	}

	// Tags
	if len(gw.Tags) > 0 {
		fmt.Printf("\nTags:\n")
		for _, t := range gw.Tags {
			fmt.Printf("  %s = %s\n", aws.ToString(t.Key), aws.ToString(t.Value))
		}
	}

	// Underlying Resources / Routes if requested
	if showResources {
		res, err := c.DescribeGatewayResources(ctx, nameOrArn)
		if err == nil && res != nil {
			if albRes, ok := res.Resources.(*types.GatewayInfrastructureResourcesMemberHostSharedAlb); ok {
				fmt.Printf("\nManaged ALBs & Infrastructure:\n")
				for i, elb := range albRes.Value {
					fmt.Printf("  ALB #%d:\n", i+1)
					fmt.Printf("    Load Balancer ARN: %s\n", aws.ToString(elb.LoadBalancerArn))
					if elb.ListenerArn != nil {
						fmt.Printf("    Listener ARN:      %s\n", aws.ToString(elb.ListenerArn))
					}
					if elb.SecurityGroup != nil {
						fmt.Printf("    Security Group:    %s\n", aws.ToString(elb.SecurityGroup))
					}
				}
			}
		}

		routes, err := c.ListGatewayRoutes(ctx, nameOrArn)
		if err == nil && len(routes) > 0 {
			fmt.Printf("\nActive Routes (Services):\n")
			for _, r := range routes {
				fmt.Printf("  - Service: %s\n", aws.ToString(r.ServiceArn))
				fmt.Printf("    Port:    %s\n", aws.ToString(r.ServiceContainerPortName))
				fmt.Printf("    Status:  %s\n", r.Status)
				if r.ServiceUrl != nil {
					fmt.Printf("    URL:     %s\n", aws.ToString(r.ServiceUrl))
				}
			}
		}
	}

	return nil
}
