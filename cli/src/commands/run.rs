use crate::commands::client::{AppClient, Res};


pub fn run(ac: &mut impl AppClient, app_name: &str, image: &str, port: u32) 
-> Res {
    ac.add_app(app_name, image, port)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::commands::client::test::{TestAppClient, AddCall};


    #[test]
    fn test_run() {
        let mut cl = TestAppClient::new();
        let app_name = "testapp";
        let app_image = "testimage";
        let port = 9090;
        let res = run(&mut cl, app_name, app_image, port).unwrap();
        
        assert_eq!(res, ());
        assert_eq!(cl.rm_counter, 0);
        assert_eq!(cl.rm_calls.len(), 0);
        assert_eq!(cl.add_counter, 1);
        assert_eq!(cl.add_calls.len(), 1);
        assert_eq!(cl.add_calls[0], AddCall{
            app_name: app_name.to_string(),
            app_image: app_image.to_string(),
            port: port,
        })
    }

}
