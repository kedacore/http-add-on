
#[macro_use] extern crate serde_derive;
#[macro_use] extern crate log;

use kube::{
    api::{ListParams},
    // api::Object,
    Client,
    Api,
};
use kube_derive::CustomResource;
use kube_runtime::controller::{Context, Controller, ReconcilerAction};
use k8s_openapi::{
    api::apps::v1::Deployment
};

use snafu::{Backtrace, /*OptionExt, ResultExt,*/ Snafu};

use tokio::time::Duration;
use futures::StreamExt;




#[derive(CustomResource, Serialize, Deserialize, Clone, Debug)]
#[kube(group = "keda.sh", version = "v1", kind = "App", namespaced)]
pub struct AppSpec {
    pub name: String,
    pub image: String,
    pub port: u32,
}

// This is a convenience alias that describes the object we get from Kubernetes
// type KubeApp = Object<App, ()>;

#[derive(Debug, Snafu)]
enum Error {
    #[snafu(display("Failed to create ConfigMap: {}", source))]
    ConfigMapCreationFailed {
        source: kube::Error,
        backtrace: Backtrace,
    },
    MissingObjectKey {
        name: &'static str,
        backtrace: Backtrace,
    },
    SerializationFailed {
        source: serde_json::Error,
        backtrace: Backtrace,
    },
}


// This is a convenience alias that describes the object we get from Kubernetes
// type KubeBook = Object<Book, ()>;
struct Data {
    client: Client,
}


#[tokio::main]
async fn main() -> Result<(), kube::Error>{
    let client = Client::try_default().await?;

    // Set a namespace. We're just hard-coding for now.
    let namespace = "default";

    let default_list_params = ListParams::default();

    let app_api: Api<App> = Api::namespaced(client.clone(), namespace);
    let deployments_api: Api<Deployment> = Api::namespaced(client.clone(), namespace);

    let context = Context::new(Data{client});
    Controller::new(app_api, default_list_params.clone())
    // TODO: app needs to own more subresources
    .owns(deployments_api, default_list_params.clone())
    .run(reconcile, error_policy, context)
    .for_each(|res| async move {
        match res {
            Ok(o) => info!("reconciled {:?}", o),
            // Err(e) => warn!("reconcile failed: {}", Report::from(e)),
            Err(_) => warn!("reconcile failed!")
        }
    })
    .await;
    

    // let newApp = App::new("myapp", AppSpec{
    //     name: String::from("MyNewApp"),
    //     image: String::from("arschles/xkcd"),
    //     port: 1234,
    // });

    

    // fooClient

    // Describe the CRD we're working with.
    // This is basically the fields from our CRD definition.
    // let resource = RawApi::customResource("books")
    //     .group("example.technosophos.com")
    //     .within(&namespace);

    // // Create our informer and start listening.
    // let informer = Informer::raw(client, resource).init().expect("informer init failed");
    // loop {
    //     informer.poll().expect("informer poll failed");

    //     // Now we just do something each time a new book event is triggered.
    //     while let Some(event) = informer.pop() {
    //         handle(event);
    //     }
    // };
    Ok(())
}

/// The controller triggers this on reconcile errors
fn error_policy(_error: &Error, _ctx: Context<Data>) -> ReconcilerAction {
    ReconcilerAction {
        requeue_after: Some(Duration::from_secs(1)),
    }
}


async fn reconcile(_: App, _: Context<Data>) -> Result<ReconcilerAction, Error> {

    // TODO: implement!

    Ok(ReconcilerAction{requeue_after: Some(Duration::from_millis(200))})
}
// fn handle(event: WatchEvent<KubeApp>) {
//     println!("Watch event for KubeApp:");
//     dbg!(event);
// }
