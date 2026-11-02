# Amazon ECS Gateway

`ecsctl` supports **Amazon ECS Gateway**, AWS's managed ingress solution for Amazon ECS. ECS Gateway automates ingress networking, TLS management, and shared Application Load Balancer lifecycle management for multi-service architectures.

---

## 🚀 Overview

Setting up ingress traffic traditionally required manually configuring Application Load Balancers (ALBs), target groups, listener rules, security groups, and DNS records for each microservice.

With **Amazon ECS Gateway**:
* **Centralized Platform Governance**: Platform teams configure the Gateway once (specifying subnets, security groups, TLS certificates, and WAF rules) and associate it with one or more ECS clusters.
* **Shared Ingress Infrastructure**: Up to **25 ECS services** can share a single dynamically provisioned ALB using host-based routing.
* **Service Self-Service**: Application teams attach their services simply by defining a `gatewayConnector` pointing to a container port.
* **Introspection & Debugging**: Inspect the underlying auto-provisioned ALBs and live service URLs directly via `ecsctl describe gateway <name> --resources`.

---

## 🛠 Managing Gateways

### 1. Create a Gateway

#### IAM Infrastructure Role (Automatic or Custom)

Amazon ECS Gateways require an IAM infrastructure role that grants ECS permission to provision and manage shared Application Load Balancers, target groups, and network interfaces on your behalf.
* **AWS Managed Policy**: `arn:aws:iam::aws:policy/AmazonECSInfrastructureRoleForGatewayHostSharedALB`
* **Trusted Service Entity**: `ecs.amazonaws.com`

> [!TIP]
> **Automatic Provisioning & Confused Deputy Protection**: If `infrastructureRoleArn` or `--infra-role-arn` is omitted, `ecsctl` automatically checks for or creates the default role `ecsInfrastructureRoleForGateways`, attaches the required managed policy, and adds the `aws:SourceAccount` condition to the trust policy to prevent the confused deputy problem.

If your organization enforces strict IAM boundary policies or separate security roles, you can pre-create the role manually:

```bash
# 1. Create the trust policy (replace <account-id> with your 12-digit AWS account ID)
cat << 'EOF' > trust-policy.json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "Service": "ecs.amazonaws.com" },
      "Action": "sts:AssumeRole",
      "Condition": {
        "StringEquals": {
          "aws:SourceAccount": "<account-id>"
        }
      }
    }
  ]
}
EOF

# 2. Create the IAM role
aws iam create-role \
  --role-name ecsInfrastructureRoleForGateways \
  --assume-role-policy-document file://trust-policy.json

# 3. Attach the AWS managed policy
aws iam attach-role-policy \
  --role-name ecsInfrastructureRoleForGateways \
  --policy-arn arn:aws:iam::aws:policy/AmazonECSInfrastructureRoleForGatewayHostSharedALB
```

#### Via YAML File (Recommended)
Create `gateway.yaml`:
```yaml
name: my-gateway
# infrastructureRoleArn is optional; ecsctl auto-provisions ecsInfrastructureRoleForGateways if omitted:
# infrastructureRoleArn: arn:aws:iam::123456789012:role/ecsInfrastructureRoleForGateways
deletionProtection: true
clusters:
  - prod-cluster
configuration:
  hostSharedAlb:
    scheme: internet-facing
    maxAlbCount: 5
    network:
      subnets:
        - subnet-0123456789abcdef0
        - subnet-0fedcba9876543210
      inboundSecurityGroup: sg-0123456789abcdef0
      ipAddressType: dualstack
    webACLArn: arn:aws:wafv2:us-east-1:123456789012:regional/webacl/prod-waf/...
tags:
  Environment: Production
```

Apply or create the gateway:
```bash
ecsctl create gateway -f gateway.yaml
# Or declaratively:
ecsctl apply -f gateway.yaml
```

#### Via Inline Flags
```bash
# Infrastructure role is automatically created if --infra-role-arn is omitted
ecsctl create gateway \
  --name my-gateway \
  --subnets subnet-0123456789abcdef0,subnet-0fedcba9876543210 \
  --security-groups sg-0123456789abcdef0 \
  --clusters prod-cluster
```

---

### 2. Connect an ECS Service to a Gateway

#### Via YAML (`service.yaml`)
Add the `gatewayConnector` block to your service configuration:
```yaml
name: web-service
cluster: prod-cluster
taskDefinition: web-app:1
desiredCount: 2
network:
  subnets:
    - subnet-0123456789abcdef0
  assignPublicIp: DISABLED
gatewayConnector:
  portName: http
  healthCheck:
    path: /healthz
    protocol: HTTP
    intervalSeconds: 15
    timeoutSeconds: 5
    healthyThresholdCount: 2
    unhealthyThresholdCount: 3
```

#### Via Inline CLI Flag
```bash
ecsctl create service \
  --name web-service \
  --cluster prod-cluster \
  --task-definition web-app:1 \
  --gateway-port http
```

---

### 3. Inspect and Debug Gateways

#### Describe Gateway Overview
```bash
ecsctl describe gateway my-gateway
```
Output:
```text
Name:                  my-gateway
ARN:                   arn:aws:ecs:us-east-1:123456789012:gateway/my-gateway
Status:                ACTIVE
Access Type:           PUBLIC
Deletion Protection:   true
Infrastructure Role:   arn:aws:iam::123456789012:role/ecsInfrastructureRoleForGateways

Configuration (HostSharedAlb):
  Scheme:              internet-facing
  Managed ALBs:        1 / 5
  Subnets:             subnet-0123456789abcdef0, subnet-0fedcba9876543210
  Inbound SG:          sg-0123456789abcdef0

Associated Clusters:
  - arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster
```

#### Inspect Provisioned ALBs and Active Service Routes (`--resources`)
```bash
ecsctl describe gateway my-gateway --resources
```
Output:
```text
Managed ALBs & Infrastructure:
  ALB #1:
    Load Balancer ARN: arn:aws:elasticloadbalancing:...:loadbalancer/app/ecs-gw-...
    Listener ARN:      arn:aws:elasticloadbalancing:...:listener/app/ecs-gw-.../443
    Security Group:    sg-0abc1234def56789

Active Routes (Services):
  - Service: arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/web-service
    Port:    http
    Status:  ACTIVE
    URL:     https://web-service.my-gateway.ecs.us-east-1.amazonaws.com
```

---

### 4. List Gateways
```bash
# Table format (default)
ecsctl list gateways

# JSON format
ecsctl list gateways --output json

# Wide output (includes ARNs)
ecsctl list gateways --wide
```

---

### 5. Associate & Disassociate Clusters
```bash
# Link gateway to an additional cluster
ecsctl gateway associate --gateway my-gateway --cluster staging-cluster

# Unlink gateway from a cluster
ecsctl gateway disassociate --gateway my-gateway --cluster staging-cluster
```

---

### 6. Delete a Gateway
```bash
# Preview what would be deleted
ecsctl delete gateway my-gateway --dry-run

# Interactive deletion (prompts to type resource name)
ecsctl delete gateway my-gateway

# Non-interactive deletion (for CI / scripts)
ecsctl delete gateway my-gateway --yes

# Alternatively via the gateway command group:
ecsctl gateway delete my-gateway --yes
```
*(Note: If deletion protection is enabled, delete with `--deletion-protection=false` or disable protection first).*
