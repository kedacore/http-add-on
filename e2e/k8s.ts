import * as env from 'env-var';
import * as short from 'short-uuid'
import * as sh from 'shelljs'
import * as fs from 'fs'
import * as tmp from 'tmp'
import { httpsoYamlTpl } from './k8s-yaml';
import * as k8s from '@kubernetes/client-node'

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

export interface App {
    namespace: string
    deployName: string
    svcName: string
    svcPort: number
}
// createApp creates a new app in the given namespace with
// the given name. the deployment and services of the app
// will be called the same thing as the name parameter
export async function createApp(
    host: string,
    namespace: string,
    name: string,
): Promise<App> {
    const port = 8080
    const kc = new k8s.KubeConfig();
    kc.loadFromDefault();

    const appsApi = kc.makeApiClient(k8s.AppsV1Api);
    await appsApi.createNamespacedDeployment(namespace, {
        metadata: {
            name: name,
            labels: {
                app: name,
            },
            },
            spec: {
                replicas: 1,
                selector: {
                    matchLabels: {
                        app: name,
                    },
                },
                template: {
                    metadata: {
                        labels: {
                            app: name,
                        },
                    },
                    spec: {
                        containers: [
                            {
                                name: name,
                                image: `arschles/xkcd`,
                                ports: [{containerPort: port}],
                            }
                        ]
                    }
                }
            }
        }
    )
    
    const coreApi = kc.makeApiClient(k8s.CoreV1Api);
    await coreApi.createNamespacedService(namespace, {
        metadata: {
            name: name,
            labels: {
                app: name,
            },
        },
        spec: {
            ports: [{port: port, targetPort: port as any}],
            selector: {
                app: name,
            },
        }
    })

    const extV1Beta1Api = kc.makeApiClient(k8s.ExtensionsV1beta1Api);
    await extV1Beta1Api.createNamespacedIngress(namespace, {
        metadata: {
            name: name,
            labels: {
                app: name,
            },
        },
        spec: {
            rules: [{
                host: host,
                http: {
                    paths: [{
                        path: `/`,
                        backend: {
                            serviceName: name,
                            servicePort: port as any,
                        }
                    }]
                }
            }]
        }
    })

    return Promise.resolve({
        namespace: namespace,
        deployName: name,
        svcName: name,
        svcPort: port
    })
}

// deleteApp deletes the app described by the given
// app parameter
export function deleteApp(app: App) {
    const ns = app.namespace
    const deplRes = sh.exec(`kubectl delete deployment -n ${ns} ${app.deployName}`)
    const svcRes = sh.exec(`kubectl delete service -n ${ns} ${app.svcName}`)
    const ingRes = sh.exec(`kubectl delete ingress -n ${ns} ${app.deployName}`)
    if(deplRes.code !== 0) {
        throw new Error(
            `error with 'kubectl delete deployment': ${deplRes.stderr}`
        )
    }
    if(svcRes.code !== 0) {
        throw new Error(
            `error with 'kubectl delete service': ${svcRes.stderr}`
        )
    }
    if(ingRes.code !== 0) {
        throw new Error(
            `error with 'kubectl delete ingress': ${ingRes.stderr}`
        )
    }
}


// createHttpScaledObject creates a new HTTPScaledObject
// in Kubernetes. The object will have the 
// parameters given.
export function createHttpScaledObject(
    host: string,
    namespace: string,
    name: string,
    deployName: string,
    svcName: string,
    port: number,
) {
    
    let yaml = httpsoYamlTpl
        .replace('{{Name}}', name)
        .replace('{{Namespace}}', namespace)
        .replace('{{DeploymentName}}', deployName)
        .replace("{{ServiceName}}", svcName)
        .replace("{{Port}}", port.toString())
        .replace("{{Host}}", host)
    const file = tmp.fileSync()
    try {
        fs.writeFileSync(file.name, yaml)
        const res = sh.exec(`kubectl apply -f ${file.name}`)
        if (res.code !== 0) {
            throw new Error(
                `error with 'kubectl apply': ${res.stderr}`
            )
        }
    } finally {
        sh.rm(file.name)
    }
}
