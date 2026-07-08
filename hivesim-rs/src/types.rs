use serde::{Deserialize, Serialize};

pub type SuiteID = u32;
pub type TestID = u32;

/// StartNodeReponse is returned by the client startup endpoint.
#[derive(Clone, Debug, Default, Serialize, Deserialize)]
pub struct StartNodeResponse {
    pub id: String, // Container ID.
    pub ip: String, // IP address in bridge network
}

/// ApiError mirrors the JSON body hive's API returns for a failed request
/// (`{"error": "..."}`, see `serveError` in `internal/libhive/api.go`). The
/// start-client endpoint returns it with a non-2xx status when the container
/// cannot be created or exits during startup (e.g. `client did not start: ...`).
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ApiError {
    pub error: String,
}

// ClientMetadata is part of the ClientDefinition and lists metadata
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ClientMetadata {
    pub roles: Vec<String>,
}

// ClientDefinition is served by the /clients API endpoint to list the available clients
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct ClientDefinition {
    pub name: String,
    pub version: String,
    pub meta: ClientMetadata,
}

#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct TestRequest {
    pub name: String,
    pub description: String,
}

/// Describes the outcome of a test.
#[derive(Clone, Debug, Default, Serialize, Deserialize)]
pub struct TestResult {
    pub pass: bool,
    pub details: String,
}
