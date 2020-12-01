use structopt::StructOpt;
use std::fmt::Debug;
mod commands;
use commands::{rm, run, client::ProdAppClient};
use reqwest::Error;
use std::result::Result;


#[derive(Debug, StructOpt)]
enum Command {
    Rm {
        app_name: String,
    },
    Run {
        app_name: String,
        #[structopt(name="image", short)]
        image: String,
        #[structopt(name="port", short)]
        port: u32,
    },
}

#[derive(Debug, StructOpt)]
#[structopt(about = "Deploy scalable, production ready containers to Kubernetes")]
struct KedaHTTP {
    #[structopt(subcommand)]
    cmd: Command,
}

#[tokio::main]
async fn main() -> Result<(), Error> {
    let keda = KedaHTTP::from_args();
    let admin_url = format!("{}/app", commands::DEPLOY_URL);
    let mut app_client = ProdAppClient::new(&admin_url);
    match keda.cmd {
        Command::Rm{app_name} => {
            match rm::rm(&mut app_client, &app_name) {
                Ok(_) => {
                    println!("Removed {}", app_name)
                },
                Err(e) => {
                    println!("Error removing app ({})", e)
                },
            }
        },
        Command::Run{app_name, image, port} => {
            match run::run(&mut app_client, &app_name, &image, port) {
                Ok(_) => {
                    println!("Deployed app")

                },
                Err(e) => {
                    println!("Error deploying ({})", e)
                },
            };
        },
    };
    Ok(())
}
