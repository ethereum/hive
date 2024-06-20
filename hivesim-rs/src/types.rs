use serde::{Deserialize, Serialize};

pub type SuiteID = u32;
pub type TestID = u32;

/// StartNodeReponse is returned by the client startup endpoint.
#[derive(Clone, Debug, Default, Serialize, Deserialize)]
pub struct StartNodeResponse {
    pub id: String, // Container ID.
    pub ip: String, // IP address in bridge network
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

#[derive(Clone, Debug)]
pub struct ContentKeyValue {
    pub key: String,
    pub value: String,
}

#[derive(Clone, Debug)]
pub struct ContentKeyOfferLookupValues {
    pub key: String,
    pub offer_value: String,
    pub lookup_value: String,
}

#[derive(Clone, Debug)]
pub enum TestData {
    /// A list of tuples containing content key/value pairs
    ContentList(Vec<ContentKeyValue>),
    /// A list of tuples containing a content key, offer value, and lookup value
    StateContentList(Vec<ContentKeyOfferLookupValues>),
}

impl TestData {
    pub fn content_list(self) -> Vec<ContentKeyValue> {
        if let TestData::ContentList(content_list) = self {
            content_list
        } else {
            panic!("TestData didn't contain ContentList: enum was likely filled with the wrong data {self:?}")
        }
    }

    pub fn state_content_list(self) -> Vec<ContentKeyOfferLookupValues> {
        if let TestData::StateContentList(state_content_list) = self {
            state_content_list
        } else {
            panic!("TestData didn't contain StateContentList: enum was likely filled with the wrong data {self:?}")
        }
    }
}
