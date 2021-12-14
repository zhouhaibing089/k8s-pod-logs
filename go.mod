module github.com/zhouhaibing089/k8s-pod-logs

go 1.16

require (
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	sigs.k8s.io/controller-runtime v0.10.3
)

require (
	github.com/itchyny/gojq v0.12.6
	github.com/minio/minio-go/v7 v7.0.20
	gopkg.in/yaml.v2 v2.4.0
)
