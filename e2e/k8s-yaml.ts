export const httpsoYamlTpl = `
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
    name: {{Name}}
    namespace: {{Namespace}}
spec:
    host: {{Host}}
    scaleTargetRef:
        deployment: {{DeploymentName}}
        service: {{ServiceName}}
        port: {{Port}}
`
