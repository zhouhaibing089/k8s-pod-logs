k8s-pod-logs
============

Backup logs from Succeeded/Failed pods.

## Save Logs to S3

Here is an example of s3 configuration file:

```yaml
endpoint: <s3 endpoint>
bucket: <bucket name>
access_key: <access key>
secret_key: <secret key>
```

And then the controller can run with:

```console
$ go run github.com/zhouhaibing089/k8s-pod-logs/cmd/controller \
    --s3-config-path=path/to/config.yaml
```

This will save the logs of Succeeded/Failed pods into s3.

## Namespace

The controller can be configured to watch a specific namespace - just add one
more flag to the controller: `--namespace=<name>`.

## Object Name

By default, the log is saved with object name as `<namespace>/<name>`. This can
be changed with flag `--log-key=<jq on pod json>`.
