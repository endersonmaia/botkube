# Increase configuration flexibility via bindings

Created on 2022-06-14 by Mateusz Szostok ([@mszostok](https://github.com/mszostok))

<!-- toc -->

- [Motivation](#motivation)
  * [Goal](#goal)
  * [Non-goal](#non-goal)
- [Proposal](#proposal)
  * [Terminology](#terminology)
  * [New syntax](#new-syntax)
- [Use cases](#use-cases)
  * [Route BotKube notifications to individual channels](#route-botkube-notifications-to-individual-channels)
  * [Route notifications to a given channel based on the Kubernetes Namespace](#route-notifications-to-a-given-channel-based-on-the-kubernetes-namespace)
  * [Send notifications to multiple communications platform](#send-notifications-to-multiple-communications-platform)
  * [Run executor only from a dedicated channel](#run-executor-only-from-a-dedicated-channel)
  * [Validate channel annotation](#validate-channel-annotation)
  * [Reference other sources inside them](#reference-other-sources-inside-them)
- [Alternatives](#alternatives)
  * [Route notifications to a given channel based on the Kubernetes Namespace](#route-notifications-to-a-given-channel-based-on-the-kubernetes-namespace-1)
    + [Annotate Namespace](#annotate-namespace)
    + [Top level Namespace property](#top-level-namespace-property)
- [Consequences](#consequences)
  * [Minimum changes](#minimum-changes)
  * [Follow-up changes](#follow-up-changes)
- [Resources](#resources)

<!-- tocstop -->

## Motivation

There is a demand in community to make the configuration more flexible in regards which notification should be collected and where to send them.

The proposed syntax not only enables new features, but also addresses the issues described in the **Configuration API syntax issues** investigation document.

### Goal

1. Routing notifications to individual channels without needing to run multiple deployments. Example use cases:
    - a separate Slack channel for sending network errors and another channel for all error events that occur in the `team-a` Namespace
    - sending Pod's error events to Slack and, at the same time, notifying Elasticsearch of all events

    Related issues:
      - [Support communications per resource group](https://github.com/infracloudio/botkube/issues/508)
      - [Support using multiple Slack channels with different configuration](https://github.com/infracloudio/botkube/issues/444)
      - [Provide multiple channels](https://github.com/infracloudio/botkube/issues/542)

2. Routing notifications to a given channel based on the Kubernetes Namespace.

    Related issues:
      - [Define the channel at the namespace level](https://github.com/infracloudio/botkube/issues/486)

3. Defining `kubectl` (executor) permissions per channel—in particular, configuring what commands can be executed and in which Namespaces.

    Currently, it's possible only if you will deploy multiple BotKube instances with different configurations.

    Related issues:
      - [Multiple Slack Channels](https://github.com/infracloudio/botkube/issues/250)

### Non-goal

Those are not the goals of this proposal. However, we should be able to implement them later.

- [Notify groups](https://github.com/infracloudio/botkube/issues/323)
- [Group error messages to thread](https://github.com/infracloudio/botkube/issues/545)
- [Customizable Messages](https://github.com/infracloudio/botkube/issues/434)
- [Support for default cluster and namespace on any Slack channels](https://github.com/infracloudio/botkube/issues/421)

## Proposal

### Terminology

| Name           | Description                                                                                                                                                            |
|----------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Executors      | Executes BotKube or `kubectl` command and sends back the result to the Bot. In the future, new executors might be added. For example, `helm`, `argo`, `istioctl`, etc. |
| Sources        | Provides domain specific notification. For example, Kubernetes events, security events (Sysdig), metrics (Prometheus), and similar.                                    |
| Communications | Describes both Bot and Sink.                                                                                                                                           |
| Bot            | Bi-directional communication such as Slack, Discord, Mattermost.                                                                                                       |
| Sink           | Unidirectional communication, such as Elasticsearch or Webhook. It may receive events from Sources only.                                                               |

![](../assets/ideation-view.png#gh-light-mode-only)
![](../assets/ideation-view-dark.png#gh-dark-mode-only)

### New syntax

This section describes the necessary changes in the syntax. **It's not backward compatible.**

1. Sources
    <table>
    <tr>
    <td> Before </td> <td> After </td>
    </tr>
    <tr>
    <td>

    ```yaml









    resources:
      - name: v1/pods
        namespaces:
          include: ["istio-system"]
        events: ["error"]
        updateSetting:
          includeDiff: true
          fields:
            - status.phase
      - name: networking.istio.io/v1alpha3/VirtualServices
        namespaces:
          include: ["all"]
        events: ["error"]

    # Recommendations about the best practices
    recommendations: true





    ```

    </td>
    <td>

    ```yaml
    sources:
      'default' # map key, name used for bindings
        kubernetes:
          # New 'namespace' property - allows Namespace restriction
          # It can be overridden in the nested level.
          namespace:
            include: ["@all"]
          resources:
            - name: v1/pods
              namespaces: # override the top level Namespace
                include: ["istio-system"]
              events: ["error"]
              updateSetting:
                includeDiff: true
                fields:
                  - status.phase
            - name: networking.istio.io/v1alpha3/VirtualServices
              # uses the default Namespace settings from top level definition.
              events: ["error"]

        # Recommendations about the best practices
        recommendations:
          image:     # "Checks if 'latest' image tag is used for container image."
            enabled: true
          pod:       # "Checks if labels are missing in the pod specs."
            enabled: true
          ingress:   # "Checks if services and tls secrets used in ingress are available."
            enabled: true
    ```

    </td>
    </tr>
    </table>

2. Executors
    <table>
    <tr>
    <td> Before </td> <td> After </td>
    </tr>
    <tr>
    <td>

    ```yaml
    settings:
      # Cluster name to differentiate incoming messages
      clustername: not-configured
      # Kubectl executor configs
      kubectl:
        enabled: false
        commands:
          verbs: ["api-resources", "...", "auth"]
          resources: ["deployments", "...", "nodes"]
    ```
    </td>
    <td>

    ```yaml
    executors:
      'kubectl-read-only' # map key, name used for bindings
        kubectl:
          # New 'namespace' property - allows Namespace restriction
          namespaces:
            include: ["team-a"]
          commands:
            verbs: ["api-resources", "...", "auth"]
            resources: ["deployments", "..", "nodes"]
    ```
    </td>
    </tr>
    </table>


3. Communications
    <table>
    <tr>
    <td> Before </td> <td> After </td>
    </tr>
    <tr>
    <td>

    ```yaml
    communications:

      slack:
        token: 'SLACK_API_TOKEN'
        # ...trimmed...


        channel: 'SLACK_CHANNEL'








      elasticsearch:
        server: 'ELASTICSEARCH_ADDRESS'
        # ...trimmed...

        # ELS index settings
        index:
          name: botkube
          type: botkube-event
          shards: 1
          replicas: 0






    ```

    </td>
    <td>

    ```yaml
    communications: # it' is a list so it allows to have e.g multiple Elasticsearch configurations
      'tenant-b-workspace': # map key
        slack:
          token: 'SLACK_API_TOKEN'
          # ...trimmed...

          # describe channel bindings
          channels:
            'default': # map key
              name: "#team-a"
              bindings:
                sources:
                  - "nodes-errors"
                  - "deprecated-api"
                executors:
                  - "kubectl-read-only"
                  - "helm-full-access"
        elasticsearch:
          server: 'ELASTICSEARCH_ADDRESS'
          # ...trimmed...

          # ELS index bindings
          index:
            'destination': # map key
              name: network-errors
              type: botkube-event
              shards: 1
              bindings:
                sources:
                  - "nodes-errors"
                  - "depreacted-api"
                # executors - not allowed in this case, ES is "sink" only.
    ```
    </td>
    </tr>
    </table>

## Use cases

This section describes example configurations that enable the requested use-cases.

### Route BotKube notifications to individual channels

With presented configuration:
- nodes errors are sent to a `#nodes` channel,
- while networks errors are sent to the `#network` channel.

**Communicators**

```yaml
communications: # allows to have multiple slacks, or ES configurations
  - name: tenant-workspace
    slack:
      token: 'SLACK_API_TOKEN'
      # customized notifications
      channels:
        'nodes':
          name: "#nodes"
          bindings:
            sources:
              - "nodes-errors"
        'network':
          name: "#network"
          bindings:
            sources:
              - "network-errors"
```

**Sources**

```yaml
sources:
 'nodes-errors': # map key, name used for bindings
   kubernetes:
     resources:
       - name: v1/nodes
         namespaces:
           include:
             - @all
         events:
           - error
  'network-errors':
    kubernetes:
      namespace:
        include: ["@all"]
      resources:
        - name: v1/pods
          namespaces: # override the top level Namespace
            include:
              - istio-system
          events:
            - error
        - name: v1/services
          events:
            - error
        - name: networking.istio.io/v1alpha3/DestinationRules
          events:
            - error
        - name: networking.istio.io/v1alpha3/VirtualServices
          events:
            - error
```

### Route notifications to a given channel based on the Kubernetes Namespace

> **Note**
>
> Currently, you can send notification to non-default channel using [annotation](https://www.botkube.io/usage/#send-notification-to-non-default-channel).
However, you need to apply `botkube.io/channel: <channel_name>` to each K8s object (Pods, Services, etc.) which is cumbersome.

With presented configuration:
- in the `#dev-team-a` channel both executors and notification sources work in the`team-a` Namespace,
- in the `#dev-team-b` channel both executors and notification sources work in the`team-b` Namespace,
- while in the `#admin` channel executors and notification sources work in the `team-a` and `team-b` Namespace.


**Communicators**
```yaml
communications: # allows to have multiple slacks, or ES configurations
  - name: tenant-workspace
    slack:
      token: 'SLACK_API_TOKEN'
      channels:
        'dev-team-a':
          name: "#dev-team-a"
          bindings:
            sources:
              - "team-a"
            executors:
              - "team-a"
        'dev-team-b':
          name: "#dev-team-b"
          bindings:
            sources:
              - "team-b"
            executors:
              - "team-b"
        'admin':
          name: "#admin"
          bindings:
            sources:
              - "team-a"
              - "team-b"
            executors:
              - "team-a"
              - "team-b"
```

**Sources**
```yaml
sources:
  'team-a':  # map key, name used for bindings
    kubernetes:
      namespaces:
        include: ["team-a"]
      resources:
        - name: v1/pods
          events:
            - create
            - delete
            - error

  'team-b':
    kubernetes:
      namespaces:
        include: ["team-b"]
      resources:
        - name: v1/pods
          events:
            - create
            - delete
            - error
```

**Executors**
```yaml
executors:
  'team-a':  # map key, name used for bindings
    kubectl:
      namespaces:
        include: ["team-b"]
      commands:
        verbs: ["get", "logs"]
        resources: ["Deployments", "Pods", "Services"]
  'team-b':
    kubectl:
      namespaces:
        include: ["team-b"]
      commands:
        verbs: ["get", "logs"]
        resources: ["Deployments", "Pods", "Services"]
```

### Send notifications to multiple communications platform

With presented configuration:
- nodes events are sent to the `#nodes` channel,
- network and nodes errors to the `#all` channel,
- and at the same time network errors are sent to Elasticsearch.

**Communicators**

```yaml
communications:
  'tenant-workspace':  # map key
    slack:
      token: 'SLACK_API_TOKEN'
      channels:
        'nodes':
          name: "#nodes"
          bindings:
            sources:
              - nodes-errors
            executors:
              - kubectl-full-access
        'all':
          name: "#all"
          bindings:
            sources:
              - network-errors
              - nodes-errors
            executors:
              - kubectl-full-access
    elasticsearch:
      server: 'ELASTICSEARCH_ADDRESS'
      indices:
        'for-network':
          name: network-errors
          type: botkube-event
          shards: 1
          bindings:
            sources:
              - network-errors
```

**Sources**
```yaml
sources:
  'nodes-errors':  # map key, name used for bindings
    kubernetes:
      resources:
        - name: v1/nodes
          namespaces:
            include: ["@all"]
          events:
            - error
  'network-errors':
    kubernetes:
      namespaces:
        include: ["@all"]
      resources:
        - name: v1/services
          events:
            - error
        - name: networking.k8s.io/v1/ingresses
          events:
            - error
```

**Executors**
```yaml
executors:
  'kubectl-full-access':  # map key, name used for bindings
    kubectl:
      namespaces:
        include: ["@all"]
      commands:
        verbs: ["get", "...", "logs"]
        resources: ["Deployments", "Pods", "Services"]
```

### Run executor only from a dedicated channel

With presented configuration:
- the `kubectl` command executed from the `"#dev-team-a"` channel can see/mutate objects from the `team-a` Namespace only.
- however, the `kubectl`command  executed from the `"#admin"` channel can see/mutate objects in all Namespaces.

**Communicators**
```yaml
communications: # allows to have multiple slacks, or ES configurations
  'tenant-workspace':  # map key
    slack:
      token: 'SLACK_API_TOKEN'
      channels:
        'dev-team-a':
          name: "#dev-team-a"
          bindings:
            executors:
              - kubectl-team-a-ns-access
        'admin':
          name: "#admin"
          bindings:
            executors:
              - kubectl-full-access
```

**Executors**
```yaml
executors:
  'kubectl-full-access':  # map key, name used for bindings
    kubectl:
      enabled: true
      namespaces:
        include: ["@all"]
      commands:
        verbs: ["get", "logs"]
        resources: ["Deployments", "Pods", "Services"]
  'kubectl-team-a-ns-access':
    kubectl:
      namespaces:
        include: ["team-a"]
      commands:
        verbs: ["get", "logs"]
        resources: ["Deployments", "Pods", "Services"]
```

### Validate channel annotation

If you use the `botkube.io/channel: <channel_name>` annotation, notifications are sent to a given channel even if not authorized.
With a new syntax, we can check if there is a matching source binding for a given channel.

### Reference other sources inside them

We won't add a dedicated support to define a new source called "all-errors" and embed "network-errors", "nodes-errors" along with other resources instead of re-declaring all again. However, the [YAML anchors](https://helm.sh/docs/chart_template_guide/yaml_techniques/#yaml-anchors) can be used to overcome this issue.

**Sources**
```yaml
sources:
  'nodes-errors':
    kubernetes:
      resources: &nodes-errors
        - name: v1/nodes
          namespaces:
            include: ["@all"]
          events:
            - error
  'network-errors':
    kubernetes:
      namespaces:
        include: ["@all"]
      resources: &network-errors
        - name: v1/services
          events:
            - error
        - name: networking.k8s.io/v1/ingresses
          events:
            - error
  'all-errors':
    kubernetes:
      namespaces:
        include: ["@all"]
      resources:
        <<: *nodes-errors
        <<: *network-errors
          - name: v1/pod
            events:
              - error
```

## Alternatives

Other approaches that I consider with explanation why I ruled them out.

<details>
  <summary>Discarded alternative</summary>


### Route notifications to a given channel based on the Kubernetes Namespace

#### Annotate Namespace

Allow to set the `botkube.io/channel: <channel_name>` on the Kubernetes Namespace object. As a result, all object's notification from annotated Namespace will be sent to a given channel. Such approach solves the problem partially. You don't need to annotate each object manually in a given Namespace. However, it's still not a part of the BotKube installation. You need to do that manually, or automate that in some way. Additionally, it's decoupled from the BotKube configuration, causing that there are multiple sources of true which you need to analyze to understand to which Namespace the notification will be sent.

#### Top level Namespace property

In the proposed solution, the **namespace** property is defined separately for executors and sources. This approach provides fine-grained configuration. You can specify allowed namespace independently, so you can watch for events in all Namespaces but allow `kubectl` usage only in `dev` Namespace.

Unfortunately it doesn't come without any cost. If you want to have a dedicated bindings for Team A, which narrows-down all `sources` and `exectors` to the `team-a` Namespace, you need to configure that multiple times. It may be error-prone.
To solve that we can extract the **namespace** property to top level. In this case it will be common for all bindings:

```yaml
communications: # allows to have multiple slacks, or ES configurations
  - name: tenant-workspace
    slack:
      token: 'SLACK_API_TOKEN'
      # customized notifications
      channels:
        - name: "#nodes"
          namespace:
            include: ["team-a"]
          bindings:
            sources:
              - "nodes-errors"
            executors:
              - "kubectl-read-only"
              - "helm-full-access"

executors:
  - name: kubectl-read-only
    kubectl:
      commands:
        verbs: ["api-resources", "...", "auth"]
        resources: ["deployments", "..", "nodes"]
  - name: helm-full-access
    helm:
      commands:
        verbs: ["list", "delete", "install"]

sources:
  - name: nodes-errors
   kubernetes:
     resources:
       - name: v1/nodes
         events:
           - error
```

## Use array instead of map

We could also use an array for defining multiple configuration. For example:

```yaml
communications:
  - name: tenant-workspace
    slack:
      token: 'SLACK_API_TOKEN'
      channels:
        - name: "#nodes"
          namespace:
            include: ["team-a"]
          bindings:
            sources:
              - "nodes-errors"
            executors:
              - "kubectl-read-only"
              - "helm-full-access"

executors:
  - name: kubectl-read-only
    kubectl:
      commands:
        verbs: ["api-resources", "...", "auth"]
        resources: ["deployments", "..", "nodes"]
  - name: helm-full-access
    helm:
      commands:
        verbs: ["list", "delete", "install"]

sources:
  - name: nodes-errors
   kubernetes:
     resources:
       - name: v1/nodes
         events:
           - error
```

It gives us more descriptive and intuitive API, however, after evaluation we found such problems:
- it's not easy to define an environment variable to override the `communications[*].slack.token` property
- Helm doesn't support overriding array parameters. It replaces the whole array. For example:
  ```yaml
  # values.yaml
  communications: # allows to have multiple slacks, or ES configurations
  - name: tenant-workspace
    slack:
      enabled: true
      token: 'SLACK_API_TOKEN'
  ```
  if installed with `helm install --set communications[0].slack.token="foo"`, the rest of the `slack` properties will be removed instead of merged. See related issue: https://github.com/helm/helm/issues/5711.
- it requires additionally validation logic, e.g. check that a given channel is not specified multiple times, otherwise we would need to add specific merging strategy. For example, replace the previous occurrence or merge related properties with append/override option.

</details>

## Consequences

This section described necessary changes if proposal will be accepted.

### Minimum changes

1. The `resources` notifications are moved under `sources[].kubernetes[].resources`.
2. Kubectl executor moved under `executors[].kubectl`.
3. The `namespaces.include` and `namespaces.exclude` properties are added to the `kubectl` executor.
4. The `namespaces.include` and `namespaces.exclude` properties are added to `sources[].kubernetes[]`.
5. The `resource_config.yaml` and `comm_config.yaml` are merged into one, but you can provide config multiple times. In the same way, as Helm takes the `values.yaml` file. It's up to the user how it will be split.
6. Update documentation about configuration.
7. Provide migration guide.

### Follow-up changes

1. Change selector for all Namespaces from `all` to `@all`.
2. Add full channel/user indicator - `@` or `#`.
3. Recommendations are merged under notifications.
4. Update `@BotKube` commands to reflect new configuration.
5. **Optional**: [Filters](https://www.botkube.io/filters/) are renamed to `sources` and configuration is added under `sources[].{name}`.

    > **Note**
    >
    > In the future, [Filters](https://www.botkube.io/filters/) should be completely removed from the BotKube and replaced with the plugin system.

7. **Optional**: Add CLI to simplify creating/updating configuration.

## Resources

- [First implementation, which was based on profiles](https://github.com/infracloudio/botkube/pull/291). Unfortunately, this pull request is too outdated, and the work would need to be started from the ground. Additionally, it doesn't address the syntax issues.
- [Root feature Epic](https://github.com/infracloudio/botkube/issues/596)
