# acl-api
API that stores rules of network to be consumed by acl-operator


# Archtecture 

```mermaid
graph TD;
    developer[Developer];
    tsuru[TSURU API];
    aclapi[ACL-API];
    mongodb[(MongoDB)];
    acl-operator1[acl-operator];
    acl-operator2[acl-operator];
    network-policies1[Kubernetes Network Policies]
    network-policies2[Kubernetes Network Policies]


    developer -- Manage ACL Rules --> tsuru;
    tsuru --> aclapi;
    aclapi --> mongodb;
    acl-operator1 -- Pull Rules ----> aclapi
    acl-operator2 -- Pull Rules ----> aclapi

        
    subgraph cluster-one
      acl-operator1 -- Manage --> network-policies1
    end
    
    subgraph cluster-N
      acl-operator2 -- Manage --> network-policies2
    end

```
