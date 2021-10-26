import * as env from 'env-var';
import * as short from 'short-uuid'
import * as sh from 'shelljs'

const shortUID = short.generate().toLowerCase()
export const namespace = `e2e-app-${shortUID}-ns`
export const appName = `e2e-app-${shortUID}-n`
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
