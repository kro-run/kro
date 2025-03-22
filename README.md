# kro | Kube Resource Orchestrator


[![Go Report Card](https://goreportcard.com/badge/github.com/kro-run/kro)](https://goreportcard.com/report/github.com/kro-run/kro)
[![unit tests](https://github.com/kro-run/kro/actions/workflows/unit-tests.yaml/badge.svg)](https://github.com/kro-run/kro/actions/workflows/unit-tests.yaml)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/kro-run/Kro)
![GitHub License](https://img.shields.io/github/license/kro-run/kro)
[![Build and Publish](https://github.com/kro-run/kro/actions/workflows/build-push-image.yaml/badge.svg?branch=main)](https://github.com/kro-run/kro/actions/workflows/build-push-image.yaml)
![GitHub Repo stars](https://img.shields.io/github/stars/kro-run/kro)

This project aims to simplify the creation and management of complex custom resources for Kubernetes.

Kube Resource Orchestrator (**kro**) helps you to define complex multi-resource constructs as reusable components in your applications and systems. It does this by providing a Kubernetes-native, vendor agnostic way to define groupings of Kubernetes resources. 

kro's fundamental custom resource is the *ResourceGraphDefinition*. A ResourceGraphDefinition defines collections of underlying Kubernetes resources. It can define any Kubernetes resources, either native or custom, and can specify the dependencies between them. This lets you define complex custom resources, and include default configurations for their use.

The kro controller will determine the dependencies between resources, establish the correct order of operations to create and configure them, and then dynamically create and manage all of the underlying resources for you.

kro is Kubernetes native and integrates seamlessly with existing tools to preserve familiar processes and interfaces.

## Documentation

| Title                                  | Description                     |
| -------------------------------------- | ------------------------------- |
| <a href="https://kro.run/docs/overview" target="_blank" rel="noopener noreferrer">Introduction</a> | An introduction to kro          |
| <a href="https://kro.run/docs/getting-started/Installation" target="_blank" rel="noopener noreferrer">Installation</a> | Install kro in your cluster     |
| <a href="https://kro.run/docs/getting-started/deploy-a-resource-graph-definition" target="_blank" rel="noopener noreferrer">Getting started</a> | Deploy your first ResourceGraphDefinition |
| <a href="https://kro.run/docs/concepts/resource-group-definitions" target="_blank" rel="noopener noreferrer">Concepts</a> | Learn more about kro concepts   |
| <a href="https://kro.run/examples/" target="_blank" rel="noopener noreferrer">Examples</a> | Example resources               |
| <a href="https://github.com/kro-run/kro/blob/main/CONTRIBUTING.md" target="_blank" rel="noopener noreferrer">Contributions</a> | How to get involved  

## FAQ

1. **What is kro?**

    Kube Resource Orchestrator (**kro**) is a new operator for Kubernetes that simplifies the creation of complex Kubernetes resource configurations.
    kro lets you create and manage custom groups of Kubernetes resources by defining them as a *ResourceGraphDefinition*, the project's fundamental custom resource.
    ResourceGraphDefinition specifications define a set of resources and how they relate to each other functionally.
    Once defined, ResourceGraphDefinitions can be applied to a Kubernetes cluster where the kro controller is running.
    Once validated by kro, you can create instances of your ResourceGraphDefinition.
    kro translates your ResourceGraphDefinition instance and its parameters into specific Kubernetes resources and configurations which it then manages for you.

2. **How does kro work?**

    kro is designed to use core Kubernetes primitives to make resource grouping, customization, and dependency management simpler.
    When a ResourceGraphDefinition is applied to the cluster, the kro controller verifies its specification, then dynamically creates a new CRD and registers it with the API server.
    kro then deploys a dedicated controller to respond to instance events on the CRD. This microcontroller is responsible for managing the lifecycle of resources defined in the ResourceGraphDefinition for each instance that is created.

3. **How do I use kro?**

    To create your custom resource groupings, you create *ResourceGraphDefinition* specifications. These specify one or more Kubernetes resources, and can include specific configurations for each resource.

    For example, you can define a *WebApp* ResourceGraphDefinition that defines a *WebAppCRD* CRD that is composed of a *Deployment*, pre-configured to deploy your web server backend, and a *Service* configured to run on a specific port.
    You can just as easily create a more complex *WebAppWithDB* ResourceGraphDefinition by combining the existing *WebApp* ResourceGraphDefinition with a *Table* custom resource to provision a cloud managed database instance for your web app to use.

    Once you have defined a ResourceGraphDefinition, you can apply it to a Kubernetes cluster where the kro controller is running. kro will take care of the heavy lifting of creating CRDs and deploying dedicated controllers in order to manage instances of your new custom resource group.

    To create an instance of your custom resource groupings, you create an instance of the CRD that your ResourceGraphDefinition specifies. In the WebApp ResourceGraphDefinition example, this would be an instance of the *WebAppCRD* CRD. kro will respond by dynamically creating, configuring, and managing the underlying Kubernetes resources for you. 

4. **Why did you build this project?**

    We want to help streamline and simplify building with Kubernetes. Building with Kubernetes means dealing with resources that need to operate and work together, and orchestrating this can get complex and difficult at scale.
   With this project, we're taking a step in reducing the complexity of resource dependency management and customization, paving the way for a simple and scalable way to create complex custom resources for Kubernetes.

5. **Can I use this in production?**

   This project is in active development and not yet intended for production use.
   The *ResourceGraphDefinition* CRD and other APIs used in this project are not yet solidified and highly subject to change.

## Community Participation

Development and discussion is coordinated in the <a href="https://communityinviter.com/apps/kubernetes/community" target="_blank" rel="noopener noreferrer">Kubernetes Slack (invite link)</a> channel <a href="https://kubernetes.slack.com/archives/C081TMY9D6Y" target="_blank" rel="noopener noreferrer">#kro</a> channel.

Please join our community meeting.
* Every other Wednesday at 9AM PT (Pacific Time). <a href="http://www.thetimezoneconverter.com/?t=9%3A00&tz=PT%20%28Pacific%20Time%29" target="_blank" rel="noopener noreferrer">Convert to local timezone</a> 
* Agenda: <a href="https://docs.google.com/document/d/1GqeHcBlOw6ozo-qS4TLdXSi5qUn88QU6dwdq0GvxRz4/edit?tab=t.0" target="_blank" rel="noopener noreferrer">Public doc</a>
* Join us: <a href="https://us06web.zoom.us/j/85388697226?pwd=9Xxz1F0FcNUq8zFGrsRqkHMhFZTpuj.1" target="_blank" rel="noopener noreferrer">Zoom meeting</a>
* Community meeting recordings:  <a href="https://www.youtube.com/channel/UCUlcI3NYq9ehl5wsdfbJzSA" target="_blank" rel="noopener noreferrer">YouTube channel</a>


[tz-help]: http://www.thetimezoneconverter.com/?t=9%3A00&tz=PT%20%28Pacific%20Time%29
[agenda]: https://docs.google.com/document/d/1GqeHcBlOw6ozo-qS4TLdXSi5qUn88QU6dwdq0GvxRz4/edit?tab=t.0
[zoom]: https://us06web.zoom.us/j/85388697226?pwd=9Xxz1F0FcNUq8zFGrsRqkHMhFZTpuj.1
[youtube]: https://www.youtube.com/channel/UCUlcI3NYq9ehl5wsdfbJzSA

## Security

See <a href="https://github.com/kro-run/kro/blob/main/CONTRIBUTING.md#security-issue-notifications" target="_blank" rel="noopener noreferrer">CONTRIBUTING</a> for more information.

## License

<a href="https://github.com/kro-run/kro/blob/main/LICENSE" target="_blank" rel="noopener noreferrer">Apache 2.0</a>
