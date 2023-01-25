# acl-api
API that stores rules of network to be consumed by acl-operator


# Architecture

```mermaid
graph TD;
    developer[Developer];
    tsuru[TSURU API];
    aclapi[ACL-API];
    mongodb[(MongoDB)];
    acl-operator[acl-operator];
    network-policies[Kubernetes Network Policies]

    developer -- Manage ACL Rules --> tsuru;
    tsuru --> aclapi;
    aclapi --> mongodb;
    acl-operator -- Pull Rules ----> aclapi

    click tsuru "https://www.github.com/tsuru/tsuru" "Access github project"
    click aclapi "https://www.github.com/tsuru/acl-api" "Access github project"

    click acl-operator "https://www.github.com/tsuru/acl-operator" "Access github project"
    click network-policies "https://kubernetes.io/docs/concepts/services-networking/network-policies/" "Read more about kubernetes network policies"

    subgraph "cluster(s) [1..N]"
      acl-operator -- Manage --> network-policies
    end

```

# concepts

## rule

Rule is a dynamic target that tsuru application connect into, rule can  translated into a firewall rules or kubernetes network policies delegating capacity to the drivers, the responsability of acl-api is to store these rules and serve as a source of truth of all network permissions.

## service instance

Tsuru API provides a contract to extend app with other apis, acl-api used this generic resource to gather many rules into one shareable resource, it means that you can add many rules into a service instance, and bind it service instance to many apps.


# artifacts

- [Docker Hub Repository](https://hub.docker.com/r/tsuru/acl-api)
