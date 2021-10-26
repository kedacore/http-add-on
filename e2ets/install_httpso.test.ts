import test from 'ava'
import * as tmp from 'tmp'
import * as fs from 'fs'
import * as sh from 'shelljs'
import { namespace, appName } from './k8s'

test.before(t => {
    // create an HTTP ScaledObject
    const tmpFile = tmp.fileSync()
    let yml = httpsoYaml
        .replace('{{Name}}', appName)
        .replace('{{Namespace}}', namespace)
        .replace('{{DeploymentName}}', appName)
        .replace("{{ServiceName}}", "testscaler")
        .replace("{{Port}}", "8080")
    fs.writeFileSync(tmpFile.name, yml)
    sh.exec(`kubectl create namespace ${namespace}`)
    t.is(
        0,
        sh.exec(`kubectl apply -f ${tmpFile.name} --namespace ${namespace}`).code,
        'creating an HTTPScaledObject should work.',
    )
})
test.after(t => {
    t.is(
        0,
        sh.exec(`kubectl delete httpscaledobject ${appName} --namespace ${namespace}`).code,
        "deleting the HTTPScaledObject should work"
    )
})

test("HTTPScaledObject install results in a ScaledObject", t => {
    let scaledObjectFound = false
    for(let i = 0; i < 20; i++) {
        let res = sh.exec(`kubectl get scaledobject --namespace ${namespace} ${appName}`)
        if(res.code === 0) {
            scaledObjectFound = true
            break
        }
        t.log(`Waiting for ${appName} to be ready...`)
        sh.exec(`sleep 1`)
    }
    t.true(
        scaledObjectFound,
        `scaled object ${appName} should have been created by the HTTP Addon operator`
    )
})


const httpsoYaml = `
kind: HTTPScaledObject
apiVersion: http.keda.sh/v1alpha1
metadata:
    name: {{Name}}
    namespace: {{Namespace}}
spec:
    scaleTargetRef:
        deployment: {{DeploymentName}}
        service: {{ServiceName}}
        port: {{Port}}
`
