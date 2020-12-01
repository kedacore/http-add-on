use crate::commands::client::{AppClient, Res};

pub fn rm(ac: &mut impl AppClient, app_name: &str)
-> Res {
    ac.rm_app(app_name)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::commands::client::test::TestAppClient;


    #[test]
    fn test_run() {
        let mut cl = TestAppClient::new();
        let res = rm(&mut cl, "testapp").unwrap();

        assert_eq!(res, ());
        assert_eq!(cl.rm_counter, 1);
        assert_eq!(cl.rm_calls.len(), 1);
        assert_eq!(cl.rm_calls[0], "testapp".to_string());
        assert_eq!(cl.add_counter, 0);
        assert_eq!(cl.add_calls.len(), 0);
    }

}
