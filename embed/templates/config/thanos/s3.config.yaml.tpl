type: S3
config:
  bucket: "{{ .Bucket }}"
  endpoint: "s3.{{ .Region }}.amazonaws.com"
  region: "{{ .Region }}"
  aws_sdk_auth: false
  access_key: "{{ .AccessKey }}"
  insecure: false
  signature_version2: false
  secret_key: " {{ .SecretKey }} "
  put_user_metadata: {}
  part_size: 67108864
