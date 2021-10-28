import test from 'ava'
import * as tmp from 'tmp'
import * as env from 'env-var';
import * as sh from 'shelljs'
import {
    namespace,
    appName,
    writeHttpScaledObject,
    createApp,
    deleteApp,
    App,
} from './k8s'
import {httpRequest} from './http'

test.beforeEach(t => {
    const tmpFile = tmp.fileSync()
    t.context["tmpFile"] = tmpFile
    const ns = namespace(), name = appName()
    t.context["ns"] = ns
    t.context["name"] = name
    const app = createApp(ns, name)
    t.context["app"] = app
    writeHttpScaledObject(tmpFile, namespace(), name, "testdeploy", "testsvc", 8080)
    let installRes = sh.exec(`kubectl apply -f ${tmpFile.name} --namespace ${ns}`)
    t.is(
        0,
        installRes.code,
        'creating an HTTPScaledObject should work.',
    )
})

// remove the HTTPScaledObject
test.afterEach(t => {
    const ns = t.context["ns"], name = t.context["name"]
    let rmRes = sh.exec(`kubectl delete httpscaledobject -n ${ns} ${name}`)
    t.is(
        0,
        rmRes.code,
        "couldn't delete HTTPScaledObject"
    )
})

// remove the app
test.afterEach(t => {
    const app = t.context["app"] as App
    deleteApp(app)
})

// remove the HTTPScaledObject YAML file
test.afterEach(t =>{
    const tmpFile = t.context["tmpFile"] as tmp.FileResult
    sh.rm(tmpFile.name)
})

test("HTTPScaledObject install results in a ScaledObject", t => {
    const ns = t.context["ns"], name = t.context["name"]
    let scaledObjectFound = false
    for(let i = 0; i < 20; i++) {
        let res = sh.exec(`kubectl get scaledobject --namespace ${ns} ${name}`)
        if(res.code === 0) {
            scaledObjectFound = true
            break
        }
        t.log(`Waiting for ${name} to be ready...`)
        sh.exec(`sleep 1`)
    }
    t.true(
        scaledObjectFound,
        `scaled object ${name} should have been created by the HTTP Addon operator`
    )
})


test("scaling up from zero should work", async t => {
    const ingress = env.get("INGRESS_ADDRESS").required().asString()
    const {status, elapsedMS} = await httpRequest(ingress)
    t.is(status, 200, "the first request should scale the app from 0")
    const maxElapsedMS = 2000
    t.true(
        elapsedMS < maxElapsedMS,
        `the first request should take less than ${maxElapsedMS}ms`
    )
})

test("servicing requests after scaled up to 1 should work", async t => {
    const ingress = env.get("INGRESS_ADDRESS").required().asString()

    // make first request
    const resp1 = await httpRequest(ingress)
    t.is(
        resp1.status,
        200,
        `first request responded with status ${resp1.status}`
    )

    // make second request immediately afterward
    const resp2 = await httpRequest(ingress)
    t.is(
        resp2.status,
        200,
        `second request responded with status ${resp2.status}`
    )
    const maxElapsedMS = 500
    t.true(
        resp2.elapsedMS < maxElapsedMS,
        `second response took ${resp2.elapsedMS}ms, which was more than the max of ${maxElapsedMS}`
    )
})
