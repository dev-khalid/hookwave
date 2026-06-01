# Sprint 6 - Kubernetes + Helm + KEDA autoscaling demo

Estimated: 12-18h. Study first: Study Guide Phase H (K8s core, kind, Helm, KEDA).

## Goal & outcome

The stack runs on a local Kubernetes cluster (`kind`) via Helm charts, and KEDA autoscales the
`processor` deployment based on ElasticMQ queue depth. You can fire a producer burst and watch processor
replicas scale up, then scale back down (or to zero) as the queue drains. This is the proof that the
design scales horizontally.

## Study first (why)

- K8s objects: Deployment, Service, ConfigMap, Secret, namespace - and `kubectl` to inspect them.
- `kind`: a throwaway cluster in Docker; how to load locally-built images into it.
- Helm: chart structure (`Chart.yaml`, `values.yaml`, `templates/`), templating, install/upgrade.
- KEDA: `ScaledObject`, polling interval, cooldown, min/max replicas, and the `aws-sqs-queue` scaler with
  `awsEndpoint` for ElasticMQ.

## Build steps (in order)

1. **Create a `kind` cluster.** Confirm `kubectl get nodes` works. Decide a namespace (e.g. `hookwave`).
2. **Build and load images into kind.** Build your Sprint 5 images, then `kind load docker-image` each so
   the cluster can run them without a registry.
3. **Write Helm charts in `deploy/helm/`.** Start with an umbrella chart and subcharts (or one chart with
   templated components) for: producer, processor, subscriber, ElasticMQ, MinIO. Each app gets a
   Deployment + Service; config via ConfigMap; creds via Secret. Mount `subscriptions.yaml` via ConfigMap.
4. **Install infra first** (ElasticMQ, MinIO), verify pods are healthy and reachable in-cluster
   (`kubectl port-forward` to check UIs). Then install the apps; verify the end-to-end flow works in-cluster
   (producer interval mode -> objects appear in MinIO).
5. **Install KEDA** into the cluster (its own Helm chart/namespace). Confirm the KEDA operator pods run.
6. **Add a `ScaledObject` for the processor** targeting the processor Deployment, with an `aws-sqs-queue`
   trigger pointed at the in-cluster ElasticMQ:

```yaml
triggers:
  - type: aws-sqs-queue
    metadata:
      queueURL: http://elasticmq:9324/000000000000/webhook-events
      awsRegion: elasticmq
      awsEndpoint: http://elasticmq:9324
      queueLength: "5"
```

   Set `minReplicaCount`/`maxReplicaCount` (e.g. 0..10) and a sensible `pollingInterval`/`cooldownPeriod`.
   Provide dummy AWS creds via a `TriggerAuthentication`/Secret since the SDK still wants them.
7. **Run the scaling demo.** Trigger the producer's burst mode (a Job or scaling the producer up briefly)
   to flood the queue. Watch `kubectl get deploy/processor -w` and `kubectl get hpa` (KEDA creates an HPA)
   scale replicas up as queue depth exceeds `queueLength`, then back down as the processor drains it.
8. **Document the demo** (commands to run, what to watch) in the chart's README so it's repeatable.

## Definition of Done

- `helm install`/`upgrade` brings the whole stack up on kind; all pods reach Ready.
- End-to-end flow works in-cluster (events stored in MinIO).
- A producer burst causes processor replicas to scale up (visible in `kubectl get deploy -w`), and they
  scale back down (or to zero) after the queue drains.
- The whole thing can be torn down (`helm uninstall`, `kind delete cluster`) and recreated from the charts.

## Pitfalls

- Forgetting to `kind load docker-image` -> pods stuck `ImagePullBackOff` (set `imagePullPolicy: IfNotPresent`).
- KEDA SQS scaler still requires AWS credentials even for ElasticMQ - supply dummy creds via a Secret +
  `TriggerAuthentication`, and set `awsRegion` to a dummy and `awsEndpoint` to the in-cluster ElasticMQ.
- Wrong `queueURL` host: inside the cluster it must use the ElasticMQ Service name, not `localhost`.
- Scaling looks "stuck" because of `pollingInterval`/`cooldownPeriod` - lower them for the demo so changes
  are visible quickly.
- ElasticMQ/MinIO data is ephemeral in pods unless you add PVCs - fine for a demo, just know it.

## After this sprint

You've built the full core. Pick from the Nice-to-have list in `architecture.md` (retries + DLQ + HMAC,
a Postgres-backed subscription API, CI, or real AWS via Terraform/EKS) as your next learning goal.

## Commit

Commit as "feat(k8s): Helm charts + KEDA queue-depth autoscaling on kind".
