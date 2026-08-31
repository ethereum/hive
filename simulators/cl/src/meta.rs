//! Run-time metadata written by each `<client>-specs` image and harvested
//! by the simulator. Each image writes one `cl-meta.json` file at `/out/`
//! after its native spec-test runner finishes, declaring the JUnit files it
//! produced and how to classify each one.

use serde::Deserialize;

#[allow(dead_code)]
#[derive(Debug, Deserialize)]
pub struct RunMeta {
    pub client: String,
    pub source_repo: String,
    pub source_ref: String,
    #[serde(default)]
    pub source_sha: String,
    #[serde(default)]
    pub client_version: String,
    pub consensus_spec_tests_ref: String,
    #[serde(default)]
    pub network: String,
    pub suites: Vec<SuiteDescriptor>,
}

#[allow(dead_code)]
#[derive(Debug, Deserialize)]
pub struct SuiteDescriptor {
    pub junit_file: String,
    #[serde(default)]
    pub project: String,
    #[serde(default)]
    pub preset: String,
    #[serde(default)]
    pub fork: String,
    #[serde(default)]
    pub category: String,
    #[serde(default)]
    pub subcategory: Option<String>,
    #[serde(default)]
    pub source_subdir: Option<String>,
}
