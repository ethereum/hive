use crate::types::TestResult;
use tokio::task::JoinError;

/// Ensures that 'name' contains the client type.
pub fn client_test_name(name: String, client_type: String) -> String {
    if name.is_empty() {
        return client_type;
    }
    if name.contains("CLIENT") {
        return name.replace("CLIENT", &client_type);
    }
    format!("{} ({})", name, client_type)
}

pub fn extract_test_results(join_handle: Result<(), JoinError>) -> TestResult {
    match join_handle {
        Ok(()) => TestResult {
            pass: true,
            details: "".to_string(),
        },
        Err(err) => {
            let err = err.into_panic();
            let err = if let Some(err) = err.downcast_ref::<&'static str>() {
                err.to_string()
            } else if let Some(err) = err.downcast_ref::<String>() {
                err.clone()
            } else {
                format!("?{:?}", err)
            };

            TestResult {
                pass: false,
                details: err,
            }
        }
    }
}
