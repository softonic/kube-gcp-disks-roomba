# kube-gcp-disks-roomba

[![Version Widget]][Version] [![License Widget]][License] [![GoReportCard Widget]][GoReportCard] [![DockerHub Widget]][DockerHub]

[Version]: https://github.com/softonic/kube-gcp-disks-roomba/releases
[Version Widget]: https://img.shields.io/github/release/softonic/kube-gcp-disks-roomba.svg?maxAge=60
[License]: http://www.apache.org/licenses/LICENSE-2.0.txt
[License Widget]: https://img.shields.io/badge/license-APACHE2-1eb0fc.svg
[GoReportCard]: https://goreportcard.com/report/softonic/kube-gcp-disks-roomba
[GoReportCard Widget]: https://goreportcard.com/badge/softonic/kube-gcp-disks-roomba
[DockerHub]: https://hub.docker.com/r/softonic/kube-gcp-disks-roomba
[DockerHub Widget]: https://img.shields.io/docker/pulls/softonic/kube-gcp-disks-roomba.svg


Script that runs as a cronjob resource in kubernetes in GKE environments.
Removes disks from GCP that are not in use, checking first if the storage class is the default (standard).
Understanding that standard storage class has reclaimPolicy Delete.

##### Install

```
GO111MODULE=on
go build .
```

##### Shell completion

##### Help

```
cleanupDisks --help
usage: cleanupDisks [<flags>] <zones>...



Flags:
  -h, --help          Show context-sensitive help
      -project        Whether to produce flatten JSON output or not.

Args:
  <zones>  Space delimited list of zones of GCP to check.
```

##### In-cluster examples:

Run `cleanupDisks` in the `monitoring` namespace and watch for `pods` in all namespaces:
```
kubectl run NAME --image=image [--env="key=value"] [--port=port] [--replicas=replicas] [--dry-run=bool] [--overrides=inline-json] [--command] -- [COMMAND] [args...]
kubectl run cleanupDisks --image=softonic/kube-gcp-disks-roomba -- -project=PROJECT_ID europe-west1-c us-west1-c 
```
