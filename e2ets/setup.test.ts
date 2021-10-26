import * as sh from 'shelljs'
import * as k8s from '@kubernetes/client-node'
import test from 'ava'
import * as env from 'env-var';
import {
    namespace,
    getDeploymentReplicas,
    kedaReleaseName,
    httpAddonReleaseName
} from './k8s'

const kc = new k8s.KubeConfig()
kc.loadFromDefault()

test.before('configureShellJs', () => {
    sh.config.silent = true
})

test.serial('verifyKubectl', t => {
    for (const command of ['kubectl']) {
        if (!sh.which(command)) {
            t.fail(`${command} is required for setup`)
        }
    }
    t.pass()
})

test.serial('verifyKubectlLoggedIn', t => {
    const cluster = kc.getCurrentCluster()
    t.truthy(cluster, 'Make sure kubectl is logged into a cluster.')
})

test.serial('getKubectlVersion', t => {
    t.log("Getting kubectl version")
    let result = sh.exec('kubectl version ')
    if (result.code !== 0) {
        t.fail('error getting Kubernetes version')
    } else {
        t.log('kubernetes version: ' + result.stdout)
        t.pass()
    }
})

test.serial('setup helm', t => {
    // check if helm is already installed.
    let result = sh.exec('helm version')
    if(result.code == 0) {
        t.pass('helm is already installed. skipping setup')
        return
    }
    t.is(0, sh.exec(`curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3`).code, 'should be able to download helm script')
    t.is(0, sh.exec(`chmod 700 get_helm.sh`).code, 'should be able to change helm script permissions')
    t.is(0, sh.exec(`./get_helm.sh`).code, 'should be able to download helm')
    t.is(0, sh.exec(`helm version`).code, 'should be able to get helm version')
})

test.serial("initHelm", t => {
    t.log("Setting up Helm")
    let result = sh.exec('helm repo add kedacore https://kedacore.github.io/charts')
    if (result.code !== 0) {
        t.fail("error adding kedacore repo. " + result)
    }
    result = sh.exec('helm repo update')
    if (result.code !== 0) {
        t.fail("error updating helm repos. " + result)
    }
})

test.serial('deployKEDA', t => {
    t.log(`Deploying KEDA to namespace ${namespace}`)
    let result = sh.exec(`helm install ${kedaReleaseName} kedacore/keda --create-namespace --namespace ${namespace} --set watchNamespace=${namespace}`)
    if (result.code !== 0) {
        t.fail("error deploying KEDA. " + result)
    }
    
    t.pass('KEDA deployed successfully')
})

test.serial('deployKEDAHttpAddon', t => {
    t.log(`Deploying KEDA HTTP Addon to namespace ${namespace}`)
    let result = sh.exec(`helm install http-add-on ${httpAddonReleaseName} kedacore/keda-add-ons-http --create-namespace --namespace ${namespace}`)
    if (result.code !== 0) {
        t.fail("error deploying KEDA HTTP Addon. " + result)
    }
    t.pass("KEDA HTTP Addon deployed successfully")
})

test.serial('verifyKeda', t => {
    let namespace = env.get('NAMESPACE').required().asString()
    let success = false
    for (let i = 0; i < 20; i++) {
        t.log(`checking Deployments try ${i}`)
        let operatorReplicas = getDeploymentReplicas(
            namespace,
            "keda-operator"
        )
        let metricsReplicas = getDeploymentReplicas(
            namespace,
            "keda-metrics-apiserver",
        )
        if(operatorReplicas > 0 && metricsReplicas > 0) {
            t.log('keda is running 1 pod for operator and 1 pod for metrics server')
            success = true
            break
        } else {
            t.log(`KEDA or KEDA HTTP Addon aren't ready. sleeping`)
            sh.exec('sleep 5s')
        }
    }

    t.true(
        success,
        'expected keda deployments to start 2 pods successfully'
    )
})

test.serial("verifyHttpAddon", t => {
    let found = false;
    for (let i = 0; i < 20; i++) {
        let operatorReplicas = getDeploymentReplicas(
            namespace, 
            "http-add-on-controller-manager"
        )
        if(operatorReplicas > 0) {
            found = true
            t.log("HTTP Addon is ready")
            break
        } else {
            t.log("HTTP Addon isn't ready, sleeping")
            sh.exec('sleep 5s')
        }
    }
    t.true(found, "expected HTTP Addon to start 1 pod successfully")
})
