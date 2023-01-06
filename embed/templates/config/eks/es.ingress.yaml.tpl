apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: es-ingress
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: "HTTPS"
    kubernetes.io/ingress.class: "nginx"
spec:
  defaultBackend:
    service:
      name: elasticsearch-master
      port:
        number: 9200
  rules:
  - http:
      paths:
      - pathType: Prefix
        path: "/"
        backend:
          service:
            name: elasticsearch-master
            port:
              number: 9200
