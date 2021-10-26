import test from 'ava'
import short from 'short-uuid'
import * as tmp from 'tmp'

let namespace = 'keda-http-addon-e2e-install-httpso'
let appName = `e2e-app-${short.generate()}`

test.before(t => {
    // create an HTTP ScaledObject
    const tmpFile = tmp.fileSync()
    let yml = deployYaml
        .replace('{{Name}}', appName)
        .replace('{{Namespace}}', namespace)
        .replace('{{DeploymentName}}', appName)
        .replace("{{ScalerAddress}}", "testscaler")
        .replace("{{Host}}", appName)
    // fs.writeFileSync(tmpFile.name, yml, base64ConStr))
})


const deployYaml = `
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
    name: {{Name}}
    namespace: {{Namespace}}
spec:
    minReplicaCount: 0
    maxReplicaCount: 2
    pollingInterval: 1
    scaleTargetRef:
        name: {{DeploymentName}}
        kind: Deployment
    triggers:
    - type: external-push
        metadata:
        scalerAddress: {{ScalerAddress}}
        host: {{Host}}
`
