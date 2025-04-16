# reports-server

![Version: 0.1.17](https://img.shields.io/badge/Version-0.1.17-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: v0.1.16](https://img.shields.io/badge/AppVersion-v0.1.16-informational?style=flat-square)

TODO

## Installing the Chart

Add `reports-server` Helm repository:

```shell
helm repo add reports-server https://kyverno.github.io/reports-server/
```

Install `reports-server` Helm chart:

```shell
helm install reports-server --namespace reports-server --create-namespace reports-server/reports-server
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Affinity |
| apiServicesManagement.enabled | bool | `true` | Create a helm hooks to delete api services on uninstall |
| apiServicesManagement.image.pullPolicy | string | `nil` | Image pull policy Defaults to image.pullPolicy if omitted |
| apiServicesManagement.image.registry | string | `"ghcr.io"` | Image registry |
| apiServicesManagement.image.repository | string | `"nirmata/kubectl"` | Image repository |
| apiServicesManagement.image.tag | string | `"1.30.2"` | Image tag Defaults to `latest` if omitted |
| apiServicesManagement.imagePullSecrets | list | `[]` | Image pull secrets |
| apiServicesManagement.installApiServices | object | `{"enabled":false,"installEphemeralReportsService":true}` | Install api services in manifest |
| apiServicesManagement.installApiServices.enabled | bool | `false` | Store reports in reports-server |
| apiServicesManagement.installApiServices.installEphemeralReportsService | bool | `true` | Store ephemeral reports in reports-server |
| apiServicesManagement.nodeAffinity | object | `{}` | Node affinity constraints. |
| apiServicesManagement.nodeSelector | object | `{}` | Node labels for pod assignment |
| apiServicesManagement.podAffinity | object | `{}` | Pod affinity constraints. |
| apiServicesManagement.podAnnotations | object | `{}` | Pod annotations. |
| apiServicesManagement.podAntiAffinity | object | `{}` | Pod anti affinity constraints. |
| apiServicesManagement.podLabels | object | `{}` | Pod labels. |
| apiServicesManagement.podSecurityContext | object | `{}` | Security context for the pod |
| apiServicesManagement.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"privileged":false,"readOnlyRootFilesystem":true,"runAsGroup":65534,"runAsNonRoot":true,"runAsUser":65534,"seccompProfile":{"type":"RuntimeDefault"}}` | Security context for the hook containers |
| apiServicesManagement.tolerations | list | `[]` | List of node taints to tolerate |
| autoscaling.enabled | bool | `false` | Enable autoscaling |
| autoscaling.maxReplicas | int | `100` | Max number of replicas |
| autoscaling.minReplicas | int | `1` | Min number of replicas |
| autoscaling.targetCPUUtilizationPercentage | int | `80` | Target CPU utilisation |
| autoscaling.targetMemoryUtilizationPercentage | string | `nil` | Target Memory utilisation |
| cloudnative-pg.crds.create | bool | `false` |  |
| commonLabels | object | `{}` | Labels to add to resources managed by the chart |
| config.db.dbNameSecretKeyName | string | `"dbname"` | The database name will be read from this `key` in the specified Secret, when `db.secretName` is set. |
| config.db.host | string | `"reports-server-cluster-rw.reports-server"` | Database host |
| config.db.hostSecretKeyName | string | `"host"` | The database host will be read from this `key` in the specified Secret, when `db.secretName` is set. |
| config.db.name | string | `"reportsdb"` | Database name |
| config.db.password | string | `"password"` | Database password |
| config.db.passwordSecretKeyName | string | `"password"` | The database password will be read from this `key` in the specified Secret, when `db.secretName` is set. |
| config.db.port | int | `5432` | Database port |
| config.db.portSecretKeyName | string | `"port"` | The database port will be read from this `key` in the specified Secret, when `db.secretName` is set. |
| config.db.secretName | string | `""` | If set, database connection information will be read from the Secret with this name. Overrides `db.host`, `db.name`, `db.user`, and `db.password`. |
| config.db.sslcert | string | `""` | Database SSL cert |
| config.db.sslkey | string | `""` | Database SSL key |
| config.db.sslmode | string | `"disable"` | Database SSL |
| config.db.sslrootcert | string | `""` | Database SSL root cert |
| config.db.user | string | `"app"` | Database user |
| config.db.userSecretKeyName | string | `"username"` | The database username will be read from this `key` in the specified Secret, when `db.secretName` is set. |
| config.etcd.enabled | bool | `true` |  |
| config.etcd.endpoints | string | `nil` |  |
| config.etcd.insecure | bool | `true` |  |
| config.etcd.storage | string | `nil` |  |
| fipsEnabled | bool | `false` |  |
| fullnameOverride | string | `""` | Full name override |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| image.registry | string | `"reg.nirmata.io"` | Image registry |
| image.repository | string | `"nirmata/reports-server"` | Image repository |
| image.tag | string | `nil` | Image tag (will default to app version if not set) |
| imagePullSecrets | list | `[]` | Image pull secrets |
| livenessProbe | object | `{"failureThreshold":10,"httpGet":{"path":"/livez","port":"https","scheme":"HTTPS"},"initialDelaySeconds":20,"periodSeconds":10}` | Liveness probe |
| metrics.enabled | bool | `true` | Enable prometheus metrics |
| metrics.serviceMonitor.additionalLabels | object | `{}` | Service monitor additional labels |
| metrics.serviceMonitor.enabled | bool | `false` | Enable service monitor for scraping prometheus metrics |
| metrics.serviceMonitor.interval | string | `""` | Service monitor scrape interval |
| metrics.serviceMonitor.metricRelabelings | list | `[]` | Service monitor metric relabelings |
| metrics.serviceMonitor.relabelings | list | `[]` | Service monitor relabelings |
| metrics.serviceMonitor.scrapeTimeout | string | `""` | Service monitor scrape timeout |
| nameOverride | string | `""` | Name override |
| nodeSelector | object | `{}` | Node selector |
| pdb | object | `{"enabled":true,"maxUnavailable":"50%","minAvailable":null}` | Using a PDB is highly recommended for highly available deployments. Defaults to enabled. The default configuration doesn't prevent disruption when using a single replica |
| pdb.enabled | bool | `true` | Enable PodDisruptionBudget |
| pdb.maxUnavailable | string | `"50%"` | maxUnavailable pods for PDB, will take precedence over minAvailable if both are defined |
| pdb.minAvailable | string | `nil` | minAvailable pods for PDB, cannot be used together with maxUnavailable |
| podAnnotations | object | `{}` | Pod annotations |
| podEnv | object | `{}` | Provide additional environment variables to the pods. Map with the same format as kubernetes deployment spec's env. |
| podSecurityContext | object | `{"fsGroup":2000}` | Pod security context |
| postgresql.auth.database | string | `"reportsdb"` |  |
| postgresql.auth.postgresPassword | string | `"reports"` |  |
| postgresql.enabled | bool | `false` | Deploy postgresql dependency chart |
| priorityClassName | string | `"system-cluster-critical"` | Priority class name |
| readinessProbe | object | `{"failureThreshold":10,"httpGet":{"path":"/readyz","port":"https","scheme":"HTTPS"},"initialDelaySeconds":30,"periodSeconds":10}` | Readiness probe |
| replicaCount | int | `1` | Number of pod replicas |
| resources.limits | string | `nil` | Container resource limits |
| resources.requests | string | `nil` | Container resource requests |
| securityContext | object | See [values.yaml](values.yaml) | Container security context |
| service.port | int | `443` | Service port |
| service.type | string | `"ClusterIP"` | Service type |
| serviceAccount.annotations | object | `{}` | Service account annotations |
| serviceAccount.create | bool | `true` | Create service account |
| serviceAccount.name | string | `""` | Service account name (required if `serviceAccount.create` is `false`) |
| tolerations | list | `[]` | Tolerations |

## Source Code

* <https://github.com/nirmata/reports-server>

## Requirements

Kubernetes: `>=1.16.0-0`

| Repository | Name | Version |
|------------|------|---------|
| https://cloudnative-pg.github.io/charts | cloudnative-pg | 0.22.0 |

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Nirmata | <cncf-kyverno-maintainers@lists.cncf.io> | <https://kyverno.io/> |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.11.0](https://github.com/norwoodj/helm-docs/releases/v1.11.0)
