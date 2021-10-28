import * as env from 'env-var';
import * as short from 'short-uuid'
import * as sh from 'shelljs'
import * as fs from 'fs'
import {FileResult} from 'tmp'

export function appName(): string {
    const shortUID = short.generate().toLowerCase()
    return `e2e-app-${shortUID}-n`
}

export function namespace(): string {
    return env.get('NAMESPACE').required().asString()
}

export const kedaReleaseName = "keda"
export const httpAddonReleaseName = "http-addon"

export function getDeploymentReplicas(
    namespace: string,
    deployName: string
) : Number {
    let result = sh.exec(
        `kubectl get deployment ${deployName} --namespace ${namespace} -o jsonpath="{.status.readyReplicas}"`
    )
    const parsedResult = parseInt(result.stdout, 10)
    if (isNaN(parsedResult)) {
        return 0
    }
    return parsedResult
}

export interface App{
    namespace: string
    deployName: string
    svcName: string
    svcPort: number
}
// createApp creates a new app in the given namespace with
// the given name. the deployment and services of the app
// will be called the same thing as the name parameter
export function createApp(namespace: string, name: string): App {
    // throw new Error("NOT YET IMPLEMENTED")
    return {
        namespace: namespace,
        deployName: name,
        svcName: name,
        svcPort: 8080
    }
}

// deleteApp deletes the app described by the given
// app parameter
export function deleteApp(app: App) {
    // throw new Error("NOT YET IMPLEMENTED")
}

// writeHTTPScaledObject writes an HTTPScaledObject
// with the parameters given
export function writeHttpScaledObject(
    tmpFile: FileResult,
    namespace: string,
    name: string,
    deployName: string,
    svcName: string,
    port: number,
) {
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
    let yml = httpsoYaml
        .replace('{{Name}}', name)
        .replace('{{Namespace}}', namespace)
        .replace('{{DeploymentName}}', deployName)
        .replace("{{ServiceName}}", svcName)
        .replace("{{Port}}", port.toString())
    fs.writeFileSync(tmpFile.name, yml)
}
