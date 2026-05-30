# Kubernetes Manifests

> Status: secondary/experimental path. These manifests are not the canonical deployment path for this repository and currently cover only the API layer; see `docs/deployment-paths.md`.

Deploy after Race safety tests pass. Requires Postgres and Redis (external or in-cluster).

## Apply order

```bash
kubectl apply -f deployment.yaml
kubectl apply -f pdb.yaml
kubectl apply -f hpa.yaml
```

## Secrets

Create `durak-secrets` before deploying:

```bash
kubectl create secret generic durak-secrets \
  --from-literal=postgres-url='postgres://user:pass@host:5432/durak?sslmode=disable' \
  --from-literal=redis-url='redis://host:6379/0' \
  --from-literal=jwt-secret='your-jwt-secret' \
  --from-literal=telegram-bot-token='your-bot-token'
```
