controller:
  ingressClassByName: true

  ingressClassResource:
    name: nginx-internal
    enabled: true
    default: false
    controllerValue: "k8s.io/ingress-nginx-internal"

  service:
    # Disable the external LB
    external:
      enabled: false

    # Enable the internal LB. The annotations are important here, without
    # these you will get a "classic" loadbalancer
    internal:
      enabled: true
      annotations:
        service.beta.kubernetes.io/aws-load-balancer-internal: "true"
        service.beta.kubernetes.io/aws-load-balancer-backend-protocol: tcp
        service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: 'true'
        service.beta.kubernetes.io/aws-load-balancer-type: alb
