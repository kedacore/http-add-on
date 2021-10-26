import * as sh from 'shelljs'
import test from 'ava'
import {appName, httpAddonReleaseName, kedaReleaseName, namespace} from './k8s'

test.before('setup shelljs', () => {
    sh.config.silent = true
})

test('removeHttpAddon', t => {
    t.log("removing HTTP Addon")
    let result = sh.exec(`helm delete -n ${namespace} ${httpAddonReleaseName}`)
    if (result.code !== 0) {
        t.fail(`error removing HTTP Addon: ${result}`)
    }
    t.pass("HTTP Addon undeployed successfully")
})
test('removeKeda', t => {
    let result = sh.exec(`helm delete -n ${namespace} ${kedaReleaseName}`)
    if (result.code !== 0) {
        t.fail(`error removing KEDA: ${result}`)
    }
    t.pass('KEDA and HTTP Addon undeployed successfully')
})

test("removeNS", t => {
    t.log(`removing test namespace ${namespace}`)
    let result = sh.exec(`kubectl delete ns ${namespace}`)
    if (result.code !== 0) {
        t.fail(`error removing namespace: ${result}`)
    }
    t.pass(`removed namespace ${namespace}`)
})
