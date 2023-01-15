antiAffinity: "soft"

# Shrink default JVM heap.
esJavaOpts: "-Xmx128m -Xms128m"

# Allocate smaller chunks of memory per pod.
resources:
  requests:
    cpu: "100m"
    memory: "512M"
  limits:
    cpu: "1000m"
    memory: "512M"

secret:
  enabled: true
  password: "1234Abcd"

volumeClaimTemplate:
  accessModes: [ "ReadWriteOnce" ]
  storageClassName: "es-gp2"
  resources:
    requests:
      storage: 100M
