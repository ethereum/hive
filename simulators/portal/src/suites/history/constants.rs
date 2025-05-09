use alloy_primitives::Bytes;
use anyhow::{anyhow, bail, ensure};
use ethportal_api::{
    types::{
        content_key::beacon::HistoricalSummariesWithProofKey,
        content_value::beacon::ForkVersionedHistoricalSummariesWithProof,
    },
    BeaconContentKey, BeaconContentValue, ContentValue, HistoryContentKey, HistoryContentValue,
};
use serde_yaml::Value;
use ssz::Decode;
use std::{
    fs::{self, File},
    io::Read,
    path::PathBuf,
    str::FromStr,
    sync::{Arc, LazyLock},
};

// trin-bridge constants
pub const TRIN_BRIDGE_CLIENT_TYPE: &str = "trin-bridge";

pub const BASE_TEST_PATH: &str = "./portal-spec-tests/tests/mainnet/history/hive";
const HISTORICAL_SUMMARIES_FILE_PREFIX: &str = "fork_digest_historical_summaries_";
const HISTORICAL_SUMMARIES_FILE_SUFFIX: &str = ".ssz_snappy";

/// The content key and value for the latest historical summaries
///
/// This is a static variable that is initialized once and can be reused
pub static LATEST_HISTORICAL_SUMMARIES: LazyLock<Arc<(BeaconContentKey, BeaconContentValue)>> =
    LazyLock::new(|| {
        let (content_key, content_value) =
            get_latest_historical_summaries().expect("Failed to load latest historical summaries");
        Arc::new((content_key, content_value))
    });

/// Get the latest historical summaries
///
/// This should only be used in LATEST_HISTORICAL_SUMMARIES
fn get_latest_historical_summaries() -> anyhow::Result<(BeaconContentKey, BeaconContentValue)> {
    let mut latest_epoch = 0;
    let mut latest_file: Option<PathBuf> = None;

    for entry in fs::read_dir(BASE_TEST_PATH)? {
        let entry = entry?;
        if entry.path().is_dir() {
            continue;
        }
        let file_name = entry.file_name();
        let file_name_str = file_name.to_string_lossy();

        if file_name_str.starts_with(HISTORICAL_SUMMARIES_FILE_PREFIX)
            && file_name_str.ends_with(HISTORICAL_SUMMARIES_FILE_SUFFIX)
        {
            let epoch_str = &file_name_str[HISTORICAL_SUMMARIES_FILE_PREFIX.len()
                ..file_name_str.len() - HISTORICAL_SUMMARIES_FILE_SUFFIX.len()];
            if let Ok(epoch) = epoch_str.parse::<u64>() {
                if epoch > latest_epoch {
                    latest_epoch = epoch;
                    latest_file = Some(entry.path());
                }
            } else {
                bail!("Unable to parse epoch from file name: {file_name_str}");
            }
        }
    }

    let Some(path) = latest_file else {
        bail!("No historical summaries files found in the directory");
    };

    let mut encoded = vec![];
    let historical_summaries = File::open(path)?.read_to_end(&mut encoded)?;
    ensure!(
        historical_summaries > 0,
        "Historical summaries file is empty",
    );

    let mut decoder = snap::read::FrameDecoder::new(&*encoded);
    let mut decoded: Vec<u8> = vec![];
    decoder.read_to_end(&mut decoded)?;

    let historical_summaries_with_proof =
        ForkVersionedHistoricalSummariesWithProof::from_ssz_bytes(&decoded).map_err(|err| {
            anyhow!("Failed to decoded ForkVersionedHistoricalSummariesWithProof {err:?}")
        })?;

    let content_key =
        BeaconContentKey::HistoricalSummariesWithProof(HistoricalSummariesWithProofKey {
            epoch: historical_summaries_with_proof
                .historical_summaries_with_proof
                .epoch,
        });
    let content_value =
        BeaconContentValue::HistoricalSummariesWithProof(historical_summaries_with_proof);

    Ok((content_key, content_value))
}

pub fn get_success_test_data() -> anyhow::Result<Vec<(HistoryContentKey, HistoryContentValue)>> {
    get_test_data("success")
}

fn get_test_data(test_case: &str) -> anyhow::Result<Vec<(HistoryContentKey, HistoryContentValue)>> {
    let mut content = vec![];

    let success_test_path = format!("{BASE_TEST_PATH}/{test_case}");
    for entry in std::fs::read_dir(success_test_path)? {
        let entry = entry?;
        let test_path = entry.path();
        if !test_path.is_file() {
            bail!("Expected a file, but found a directory: {test_path:?}");
        }

        let values = std::fs::read_to_string(test_path)?;
        let values: Value = serde_yaml::from_str(&values)?;
        let test_case = values
            .as_sequence()
            .expect("unable to convert test data to sequence")
            .iter()
            .map(|value| {
                let content_key: HistoryContentKey =
                    serde_yaml::from_value(value["content_key"].clone())
                        .expect("to find content key");
                let raw_content_value = Bytes::from_str(
                    value["content_value"]
                        .as_str()
                        .expect("to find content value"),
                )
                .expect("unable to convert content value to bytes");
                let content_value =
                    HistoryContentValue::decode(&content_key, raw_content_value.as_ref())
                        .expect("unable to decode content value");
                (content_key, content_value)
            })
            .collect::<Vec<_>>();

        content.extend(test_case);
    }

    Ok(content)
}

lazy_static::lazy_static! {
   pub static ref HEADER_WITH_PROOF_KEY: HistoryContentKey = {
       let value: HistoryContentKey = serde_json::from_str(
           "\"0x00720704f3aa11c53cf344ea069db95cecb81ad7453c8f276b2a1062979611f09c\""
       ).expect("unable to convert content key to HistoryContentKey");
       value
   };
   pub static ref HEADER_WITH_PROOF_VALUE: Bytes = Bytes::from_str(
       "0x080000002d020000f90222a02c58e3212c085178dbb1277e2f3c24b3f451267a75a234945c1581af639f4a7aa058a694212e0416353a4d3865ccf475496b55af3a3d3b002057000741af9731919400192fb10df37c9fb26829eb2cc623cd1bf599e8a067a9fb631f4579f9015ef3c6f1f3830dfa2dc08afe156f750e90022134b9ebf6a018a2978fc62cd1a23e90de920af68c0c3af3330327927cda4c005faccefb5ce7a0168a3827607627e781941dc777737fc4b6beb69a8b139240b881992b35b854eab9010000200000400000001000400080080000000000010004010001000008000000002000110000000000000090020001110402008000080208040010000000a8000000000000000000210822000900205020000000000160020020000400800040000000000042080000000400004008084020001000001004004000001000000000000001000000110000040000010200844040048101000008002000404810082002800000108020000200408008000100000000000000002020000b00010080600902000200000050000400000000000000400000002002101000000a00002000003420000800400000020100002000000000000000c00040000001000000100187327bd7ad3116ce83e147ed8401c9c36483140db184627d9afa9a457468657265756d50504c4e532f326d696e6572735f55534133a0f1a32e24eb62f01ec3f2b3b5893f7be9062fbf5482bc0d490a54352240350e26882087fbb243327696851aae1651b60cc53ffa2df1bae1550a0000000000000000000000000000000000000000000063d45d0a2242d35484f289108b3c80cccf943005db0db6c67ffea4c4a47fd529f64d74fa6068a3fd89a2c0d9938c3a751c4706d0b0e8f99dec6b517cf12809cb413795c8c678b3171303ddce2fa1a91af6a0961b9db72750d4d5ea7d5103d8d25f23f522d9af4c13fe8ac7a7d9d64bb08d980281eea5298b93cb1085fedc19d4c60afdd52d116cfad030cf4223e50afa8031154a2263c76eb08b96b5b8fdf5e5c30825d5c918eefb89daaf0e8573f20643614d9843a1817b6186074e4e53b22cf49046d977c901ec00aef1555fa89468adc2a51a081f186c995153d1cba0f2887d585212d68be4b958d309fbe611abe98a9bfc3f4b7a7b72bb881b888d89a04ecfe08b1c1a48554a48328646e4f864fe722f12d850f0be29e3829d1f94b34083032a9b6f43abd559785c996229f8e022d4cd6dcde4aafcce6445fe8743e1fcbe8672a99f9d9e3a5ca10c01f3751d69fbd22197f0680bc1529151130b22759bf185f4dbce357f46eb9cc8e21ea78f49b298eea2756d761fe23de8bea0d2e15aed136d689f6d252c54ebadc3e46b84a397b681edf7ec63522b9a298301084d019d0020000000000000000000000000000000000000000000000000000000000000"
   ).expect("unable to convert content value to bytes");
}
