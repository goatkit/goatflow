# Kubernetes Deployment Guide

GoatFlow ships a Helm chart in the repository at `charts/goatflow/` (backend, database,
frontend, ingress and secrets templates; see [charts/goatflow/README.md](../../charts/goatflow/README.md)).

## Helm Installation

The chart is not published to an external repository - install it from the checkout:

```bash
git clone https://github.com/goatkit/goatflow.git
cd goatflow

# MySQL (default)
helm install goatflow ./charts/goatflow --namespace goatflow --create-namespace

# PostgreSQL
helm install goatflow ./charts/goatflow --namespace goatflow --create-namespace \
  -f charts/goatflow/values-postgresql.yaml
```

Set the image tag (`backend.image.tag`) and database/secret values for your environment.
The upstream image is `ghcr.io/goatkit/goatflow`.

> **Status:** the chart works for a single-namespace deployment (verified against
> microk8s); advanced topics below are on the roadmap.

## Roadmap

- Prerequisites and requirements
- Kubernetes cluster setup (EKS, GKE, AKS, self-managed)
- Namespace configuration
- ConfigMaps and Secrets management
- Deployment manifests
- Service definitions
- Ingress configuration
- Persistent volume claims
- StatefulSets for databases
- Horizontal Pod Autoscaling (HPA)
- Vertical Pod Autoscaling (VPA)
- Helm chart installation
- GitOps with ArgoCD/Flux
- Multi-region deployment
- Service mesh integration (Istio/Linkerd)
- Monitoring with Prometheus/Grafana
- Logging with ELK/Loki
- Backup strategies
- Disaster recovery
- Security policies and RBAC
- Cost optimization

## See Also

- [Docker Deployment](docker.md)
- [Architecture Overview](../ARCHITECTURE.md)

---

*For architecture details, see [ARCHITECTURE.md](../ARCHITECTURE.md)*