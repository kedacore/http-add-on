use reqwest::{Error};
use std::result::Result;
use std::collections::HashMap;

pub type Res = Result<(), Error>;

pub trait AppClient {
    // these have to be &mut self because the TestAppClient needs
    // to modify internal state. The production implementation
    // doesn't need to do so, so it would be nice to figure out
    // how to make this just "self" and deal with the test app client
    // in a different way
    fn add_app(&mut self, app_name: &str, app_image: &str, port: u32) -> Res;
    fn rm_app(&mut self, app_name: &str) -> Res;
}

pub struct ProdAppClient {
    base_deploy_url: String
}

impl ProdAppClient {
    pub fn new(base_deploy_url: &str) -> ProdAppClient {
        ProdAppClient{
            base_deploy_url: base_deploy_url.to_string(),
        }
    }
}

impl AppClient for ProdAppClient {
    fn add_app(&mut self, app_name: &str, app_image: &str, port: u32)
    -> Res {
        let port_string = port.to_string();
        let client = reqwest::blocking::Client::new();
        let mut map = HashMap::new();
        map.insert("name", app_name);
        map.insert("image", &app_image);
        map.insert("port", &port_string);
    
        let request_url = format!("{}?name={}", self.base_deploy_url, app_name);
        client.post(&request_url)
        .json(&map)
        .send()
        .map(|_| ())
    }

    fn rm_app(&mut self, app_name: &str)
    -> Res {
        let client = reqwest::blocking::Client::new();
        let mut map = HashMap::new();
        map.insert("name", app_name);
        
        let request_url = format!("{}?name={}", self.base_deploy_url, app_name);
        client.delete(&request_url)
        .json(&map)
        .send()
        .map(|_| ())
    }
}

pub mod test {
    use super::{Res, AppClient};
    use std::vec::Vec;

    #[derive(Debug, PartialEq)]
    pub struct AddCall {
        pub app_name: String,
        pub app_image: String,
        pub port: u32,
    }

    pub struct TestAppClient {
        pub add_counter: u32,
        pub add_calls: Vec<AddCall>,
        pub rm_counter: u32,
        pub rm_calls: Vec<String>,

        // TODO: this is for stubbing out return values
        // add_return: Result<(), Error>,
        // rm_return: Result<(), Error>,
    }

    impl TestAppClient {
        pub fn new()
        ->TestAppClient {
            TestAppClient{
                add_counter: 0,
                add_calls: vec![],
                rm_counter: 0,
                rm_calls: vec![],
            }
        }
    }

    impl AppClient for TestAppClient {
        fn add_app(&mut self, app_name: &str, app_image: &str, port: u32)
        -> Res {
            self.add_counter+=1;
            self.add_calls.push(AddCall{
                app_name: app_name.to_string(),
                app_image: app_image.to_string(),
                port: port,
            });
            Ok(())
        }
        fn rm_app(&mut self, app_name: &str)
        -> Res {
            self.rm_counter+=1;
            self.rm_calls.push(app_name.to_string());
            Ok(())
        }
    }
}
