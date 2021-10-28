import test from 'ava'
import * as tmp from 'tmp'
import * as env from 'env-var';
import * as sh from 'shelljs'
import {
    namespace,
    appName,
    createHttpScaledObject,
    createApp,
    deleteApp,
    App,
    getHTTPScaledObject,
} from './k8s'
import {httpRequest} from './http'

// create an app then submit a HTTPScaledObject
// for it. these need to be done in serial, but they
// can mostly be reversed in parallel.
test.beforeEach(t => {
    const ns = namespace(), name = appName(), host = appName()
    t.context["ns"] = ns
    t.context["name"] = name
    t.context["host"] = host
    const app = createApp(host, ns, name)
    t.context["app"] = app
    createHttpScaledObject(
        host,
        namespace(),
        name,
        "testdeploy",
        "testsvc",
        8080
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
    const {status, elapsedMS} = await httpRequest(
        ingress,
        {"Host": t.context["host"]},
    )
    t.is(status, 200, "the first request should scale the app from 0")
    const maxElapsedMS = 2000
    t.true(
        elapsedMS < maxElapsedMS,
        `the first request should take less than ${maxElapsedMS}ms`
    )
})

test("duplicate hosts should fail", async t => {
    const ns = t.context["ns"], name = t.context["name"], host = t.context["host"]
    const name2 = `${name}-2`
    await createHttpScaledObject(
        host,
        namespace(),
        name2,
        "testdeploy",
        "testsvc",
        8080
    )
    const httpso = await getHTTPScaledObject(ns, name)
    // TODO: ensure there is status in the scaled object...
})

test("servicing requests after scaled up to 1 should work", async t => {
    const ingress = env.get("INGRESS_ADDRESS").required().asString()

    // make first request
    const resp1 = await httpRequest(
        ingress,
        {"Host": t.context["host"]},
    )
    t.is(
        resp1.status,
        200,
        `first request responded with status ${resp1.status}`
    )

    // make second request immediately afterward
    const resp2 = await httpRequest(
        ingress,
        {"Host": t.context["host"]},
    )
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
