# KRO (Kube Resource Orchestrator)

**KRO** is a Kubernetes-native tool that lets you create reusable APIs for
deploying multiple resources as a single unit. It transforms complex Kubernetes
deployments into simple, reusable APIs that your teams can use. By handling
resource dependencies and configuration under the hood, **KRO** lets you focus
on defining your application's structure while ensuring consistent deployments.

## Overview

**KRO** helps platform teams create standardized APIs that development teams can
use to deploy their applications. With **KRO**, you can:

- Turn complex multi-resource applications into simple APIs
- Let teams self-serve while enforcing best practices
- Keep your resources in sync with their desired state

## Quick Start

### Installation

```bash
# Fetch the latest release version
export KRO_VERSION=$(curl -s \
    https://api.github.com/repos/awslabs/kro/releases/latest | \
    grep '"tag_name":' | \
    sed -E 's/.*"([^"]+)".*/\1/' \
  )

# Install KRO using Helm
helm install kro oci://public.ecr.aws/kro/kro \
  --namespace kro \
  --create-namespace \
  --version=${KRO_VERSION}
```

### Sample Example

KRO has only one fundamental concept - the ResourceGroup CRD. A ResourceGroup
lets you define a new API that creates multiple resources together. When you
create a ResourceGroup, KRO generates a new custom API in your cluster and
configures itself to watch for instances of this API. Other teams can then use
this API to deploy resources in a consistent, controlled way.

Here's a ResourceGroup that creates a new API for deploying web applications:

```yaml
apiVersion: kro.run/v1alpha1
kind: ResourceGroup
metadata:
  name: example-web-app
spec:
  # Define what users can configure when using your API
  schema:
    apiVersion: v1alpha1
    kind: DeploymentService
    spec:
      name: string
      image: string | default="nginx"
    status:
      availableReplicas: ${deployment.status.availableReplicas}
  # Define what resources your API will create
  resources:
    - name: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: ${schema.spec.name}
        spec:
          replicas: 3
          selector:
            matchLabels:
              app: ${schema.spec.name}
          template:
            metadata:
              labels:
                app: ${schema.spec.name}
            spec:
              containers:
                - name: ${schema.spec.name}
                  image: ${schema.spec.image}
                  ports:
                    - containerPort: 80
    - name: service
      template:
        apiVersion: v1
        kind: Service
        metadata:
          name: ${schema.spec.name}
        spec:
          selector:
            app: ${schema.spec.name}
          ports:
            - protocol: TCP
              port: 80
              targetPort: 80
```

2. Use your new API:

```yaml
apiVersion: v1alpha1
kind: DeploymentService
metadata:
  name: my-app
spec:
  name: web-app
  image: nginx:latest
```

## Documentation

- [Getting Started Guide](https://kro.run/docs/getting-started)
- [API Reference](https://kro.run/docs/api)
- [Full Documentation](https://kro.run/docs)

## Community

We welcome contributions and feedback from the community!

- [Contributing Guide](https://github.com/awslabs/kro/blob/main/CONTRIBUTING.md)
- [Report Issues](https://github.com/awslabs/kro/issues)
- Community Meetings TBD

## Status

KRO is currently in alpha. While the core functionality is stable, APIs might
change as we gather feedback from the community.

Check out our [roadmap](https://github.com/orgs/awslabs/projects/181) for
upcoming features and improvements.

## Security

See [CONTRIBUTING](CONTRIBUTING.md#security-issue-notifications) for more
information.

## License

KRO is licensed under the Apache License 2.0. See [LICENSE](LICENSE) for the
full license text.
