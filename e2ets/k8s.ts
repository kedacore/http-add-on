import * as env from 'env-var';
import short from 'short-uuid'
import * as sh from 'shelljs'

export const namespace = env.get('NAMESPACE').required().asString()
export const appName = `e2e-app-${short.generate()}`
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
