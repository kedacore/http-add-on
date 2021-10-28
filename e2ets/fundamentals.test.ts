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
import axios from 'axios'

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

    const start = Date.now()
    // const res = await fetch(ingress)
    const res = await axios.get(ingress)
    const elapsed = Date.now() - start
    t.is(res.status, 200, "the first request should scale the app from 0")
    const maxElapsed = 2000
    t.true(elapsed < maxElapsed, `the first request should take less than ${maxElapsed}ms`)
})
