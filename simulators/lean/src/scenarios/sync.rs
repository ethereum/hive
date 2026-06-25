use crate::utils::helper::{
    start_bad_checkpoint_peer, start_checkpoint_sync_client_context,
    start_checkpoint_sync_helper_mesh, start_client_under_test_with_retry,
    start_post_genesis_sync_context,
    start_post_genesis_sync_context_with_extra_bootnodes_after_helper_agreement,
    HelperGossipForkDigestProfile, PostGenesisSyncContext, PostGenesisSyncTestData,
    RunningBadCheckpointPeer, LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0,
};
use crate::utils::util::{
    bootnode_metadata_for_client, current_unix_time, default_genesis_time, fork_choice_head_slot,
    lean_bootnodes_for_client, lean_client_kind, lean_clients, lean_environment,
    run_data_test_with_timeout, selected_lean_devnet, try_load_fork_choice_response,
    ClientUnderTestRole, ForkChoiceResponse, TimedDataTestSpec,
};
use alloy_primitives::B256;
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, Test};
use std::collections::HashMap;
use std::time::Duration;
use tokio::time::{sleep, timeout, Instant};

const BOOTNODES_ENVIRONMENT_VARIABLE: &str = "HIVE_BOOTNODES";
const CLIENT_PRIVATE_KEY_ENVIRONMENT_VARIABLE: &str = "HIVE_CLIENT_PRIVATE_KEY";
const IS_AGGREGATOR_ENVIRONMENT_VARIABLE: &str = "HIVE_IS_AGGREGATOR";
const LEAN_CLIENT_RUNTIME_ROLE_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_CLIENT_RUNTIME_ROLE";
const LEAN_ATTESTATION_COMMITTEE_COUNT_ENVIRONMENT_VARIABLE: &str =
    "HIVE_ATTESTATION_COMMITTEE_COUNT";
const LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_GENESIS_TIME";
const LEAN_GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_VALIDATOR_COUNT";
const LEAN_VALIDATOR_INDEX_ENVIRONMENT_VARIABLE: &str = "HIVE_VALIDATOR_INDEX";
const LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_VALIDATOR_INDICES";
const NODE_ID_ENVIRONMENT_VARIABLE: &str = "HIVE_NODE_ID";

const CHECKPOINT_SYNC_HELPER_MESH_TIMEOUT_SECS: u64 = 420;
const CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS: u64 = 180;
const HEAD_SYNC_TIMEOUT_SECS: u64 = CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS;
const HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS: u64 = 600;
const HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS: u64 = 420;
const BAD_CHECKPOINT_PEER_AHEAD_TIMEOUT_SECS: u64 = 120;
const LONG_SYNC_OUTER_TIMEOUT_CAP_SECS: u64 = 15 * 60;
const HEAD_SYNC_WITH_CLIENT_RESTART_TIMEOUT_SECS: u64 = 2 * HEAD_SYNC_TIMEOUT_SECS;
const CHECKPOINT_SYNC_FRESH_START_OUTER_TIMEOUT_SECS: u64 =
    CHECKPOINT_SYNC_HELPER_MESH_TIMEOUT_SECS + CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS;
const LATE_JOINER_HELPER_MESH_SUMMED_TIMEOUT_SECS: u64 = CHECKPOINT_SYNC_HELPER_MESH_TIMEOUT_SECS
    + CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS
    + HEAD_SYNC_WITH_CLIENT_RESTART_TIMEOUT_SECS
    + LATE_JOINER_FINALIZATION_TIMEOUT_SECS;
const LATE_JOINER_SAME_CLIENT_OUTER_TIMEOUT_SECS: u64 = ((SAME_CLIENT_GENESIS_BOOTSTRAP_PEER_COUNT
    + 1) as u64
    * CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS)
    + LATE_JOINER_FINALIZATION_TIMEOUT_SECS;
const HEAD_RECOVERY_SUMMED_TIMEOUT_SECS: u64 = HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS
    + HEAD_SYNC_WITH_CLIENT_RESTART_TIMEOUT_SECS
    + HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS
    + HEAD_SYNC_TIMEOUT_SECS;
const HEAD_BEHIND_FINALIZED_RECOVERY_SUMMED_TIMEOUT_SECS: u64 =
    HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS
        + HEAD_SYNC_WITH_CLIENT_RESTART_TIMEOUT_SECS
        + HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS
        + HEAD_SYNC_TIMEOUT_SECS;
const BAD_CHECKPOINT_REJECTION_SUMMED_TIMEOUT_SECS: u64 = HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS
    + HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS
    + HEAD_SYNC_WITH_CLIENT_RESTART_TIMEOUT_SECS
    + BAD_CHECKPOINT_PEER_AHEAD_TIMEOUT_SECS
    + HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS
    + BAD_CHECKPOINT_PEER_AHEAD_TIMEOUT_SECS
    + HEAD_SYNC_TIMEOUT_SECS
    + BAD_CHECKPOINT_PEER_AHEAD_TIMEOUT_SECS;
const LATE_JOINER_HELPER_MESH_OUTER_TIMEOUT_SECS: u64 =
    capped_long_sync_timeout_secs(LATE_JOINER_HELPER_MESH_SUMMED_TIMEOUT_SECS);
const HEAD_RECOVERY_OUTER_TIMEOUT_SECS: u64 =
    capped_long_sync_timeout_secs(HEAD_RECOVERY_SUMMED_TIMEOUT_SECS);
const HEAD_BEHIND_FINALIZED_RECOVERY_OUTER_TIMEOUT_SECS: u64 =
    capped_long_sync_timeout_secs(HEAD_BEHIND_FINALIZED_RECOVERY_SUMMED_TIMEOUT_SECS);
const BAD_CHECKPOINT_REJECTION_OUTER_TIMEOUT_SECS: u64 =
    capped_long_sync_timeout_secs(BAD_CHECKPOINT_REJECTION_SUMMED_TIMEOUT_SECS);
const BAD_CHECKPOINT_PEER_MIN_SLOT_LEAD: u64 = 4;
const SYNC_HELPER_PEER_COUNT: usize = 2;
const SAME_CLIENT_GENESIS_BOOTSTRAP_PEER_COUNT: usize = 2;
const SAME_CLIENT_GENESIS_NODE_COUNT: usize = 3;
const SAME_CLIENT_GENESIS_VALIDATOR_COUNT: usize = 3;
const SAME_CLIENT_GENESIS_VALIDATOR_NODE_INDEX: usize = 0;
const SAME_CLIENT_GENESIS_LATE_JOINER_INDEX: usize = 2;
const SAME_CLIENT_GENESIS_LATE_START_AFTER_GENESIS_SECS: u64 = 20;
const SAME_CLIENT_GENESIS_ATTESTATION_COMMITTEE_COUNT: usize = 1;
const LEAN_CLIENT_P2P_PORT: u16 = 9000;
const HEAD_SYNC_MAX_SLOT_LAG: u64 = 2;
const PRE_PAUSE_MIN_CLIENT_HEAD_SLOT: u64 = 4;
const LATE_JOINER_FINALIZATION_TIMEOUT_SECS: u64 = 300;
const LATE_JOINER_MAX_FINALIZED_SLOT_LAG: u64 = 4;
const LATE_JOINER_POLL_INTERVAL_SECS: u64 = 2;

const fn capped_long_sync_timeout_secs(summed_timeout_secs: u64) -> u64 {
    if summed_timeout_secs > LONG_SYNC_OUTER_TIMEOUT_CAP_SECS {
        LONG_SYNC_OUTER_TIMEOUT_CAP_SECS
    } else {
        summed_timeout_secs
    }
}

struct LateJoinerObservation {
    source: Option<ForkChoiceResponse>,
    client: Option<ForkChoiceResponse>,
    source_error: Option<String>,
    client_error: Option<String>,
}

#[derive(Clone)]
struct SameClientLateJoinerTestData {
    client_under_test: ClientDefinition,
    genesis_time: u64,
}

struct RunningSameClientGenesisNode {
    node_id: String,
    client: Client,
    bootnode: String,
}

struct HeadSyncObservation {
    source_before_head: B256,
    source_before_head_slot: u64,
    source_before_justified_slot: u64,
    source_before_finalized_slot: u64,
    client_head: B256,
    client_head_slot: u64,
    client_justified_slot: u64,
    client_finalized_slot: u64,
    source_after_head: B256,
    source_after_head_slot: u64,
    source_after_justified_slot: u64,
    source_after_finalized_slot: u64,
}

fn capture_head_sync_observation(
    source_before: &ForkChoiceResponse,
    client: &ForkChoiceResponse,
    source_after: &ForkChoiceResponse,
) -> HeadSyncObservation {
    HeadSyncObservation {
        source_before_head: source_before.head,
        source_before_head_slot: fork_choice_head_slot(source_before),
        source_before_justified_slot: source_before.justified.slot,
        source_before_finalized_slot: source_before.finalized.slot,
        client_head: client.head,
        client_head_slot: fork_choice_head_slot(client),
        client_justified_slot: client.justified.slot,
        client_finalized_slot: client.finalized.slot,
        source_after_head: source_after.head,
        source_after_head_slot: fork_choice_head_slot(source_after),
        source_after_justified_slot: source_after.justified.slot,
        source_after_finalized_slot: source_after.finalized.slot,
    }
}

fn head_slot_is_caught_up(client_head_slot: u64, source_head_slot: u64) -> bool {
    client_head_slot >= source_head_slot.saturating_sub(HEAD_SYNC_MAX_SLOT_LAG)
}

fn client_caught_up_to_source(source: &ForkChoiceResponse, client: &ForkChoiceResponse) -> bool {
    client.head == source.head
        || head_slot_is_caught_up(fork_choice_head_slot(client), fork_choice_head_slot(source))
}

fn source_head_reached_minimum_slot(source: &ForkChoiceResponse, minimum_slot: u64) -> bool {
    fork_choice_head_slot(source) >= minimum_slot
}

fn helper_failed_after_test_start_message(client_name: &str, phase: &str, error: &str) -> String {
    format!(
        "Local LeanSpec helper failed after client under test {client_name} was started while {phase}; test result is indeterminate and should not be interpreted as a client failure. Helper failure: {error}"
    )
}

enum HeadSyncWaitError {
    RetryableClient(String),
    NonRetryable(String),
}

impl HeadSyncWaitError {
    fn into_message(self) -> String {
        match self {
            Self::RetryableClient(message) | Self::NonRetryable(message) => message,
        }
    }
}

async fn try_wait_for_client_to_reach_source_head(
    context: &mut PostGenesisSyncContext,
    minimum_source_head_slot: u64,
    minimum_source_finalized_slot: u64,
    minimum_client_head_slot: u64,
) -> Result<(ForkChoiceResponse, ForkChoiceResponse), HeadSyncWaitError> {
    let client_name = context.client_under_test.kind.clone();
    let mut last_observation = None;
    let mut last_source_below_minimum = None;
    let mut last_client_error = None;
    let deadline = Instant::now() + Duration::from_secs(HEAD_SYNC_TIMEOUT_SECS);

    while Instant::now() < deadline {
        let source_before_fork_choice =
            context
                .try_load_live_helper_fork_choice()
                .await
                .map_err(|err| {
                    HeadSyncWaitError::NonRetryable(helper_failed_after_test_start_message(
                        &client_name,
                        "reading the helper live head before comparing client catch-up",
                        &err,
                    ))
                })?;
        if !source_head_reached_minimum_slot(&source_before_fork_choice, minimum_source_head_slot)
            || source_before_fork_choice.finalized.slot < minimum_source_finalized_slot
        {
            last_source_below_minimum = Some(source_before_fork_choice);
            sleep(Duration::from_secs(1)).await;
            continue;
        }

        let client_fork_choice =
            match try_load_fork_choice_response(&context.client_under_test).await {
                Ok(fork_choice) => fork_choice,
                Err(err) => {
                    last_client_error = Some(err);
                    sleep(Duration::from_secs(1)).await;
                    continue;
                }
            };
        let client_head_slot = fork_choice_head_slot(&client_fork_choice);
        if client_head_slot >= minimum_client_head_slot
            && client_caught_up_to_source(&source_before_fork_choice, &client_fork_choice)
        {
            return Ok((source_before_fork_choice, client_fork_choice));
        }

        let source_after_fork_choice =
            context
                .try_load_live_helper_fork_choice()
                .await
                .map_err(|err| {
                    HeadSyncWaitError::NonRetryable(helper_failed_after_test_start_message(
                        &client_name,
                        "reading the helper live head after comparing client catch-up",
                        &err,
                    ))
                })?;
        if !source_head_reached_minimum_slot(&source_after_fork_choice, minimum_source_head_slot)
            || source_after_fork_choice.finalized.slot < minimum_source_finalized_slot
        {
            last_source_below_minimum = Some(source_after_fork_choice);
            sleep(Duration::from_secs(1)).await;
            continue;
        }

        last_observation = Some(capture_head_sync_observation(
            &source_before_fork_choice,
            &client_fork_choice,
            &source_after_fork_choice,
        ));
        if client_head_slot >= minimum_client_head_slot
            && client_caught_up_to_source(&source_after_fork_choice, &client_fork_choice)
        {
            return Ok((source_after_fork_choice, client_fork_choice));
        }

        sleep(Duration::from_secs(1)).await;
    }

    let Some(observation) = last_observation else {
        if let Some(source) = last_source_below_minimum {
            return Err(HeadSyncWaitError::NonRetryable(format!(
                "Local LeanSpec helper never exposed a live head at or above required slot {} with finalized slot at or above {} within {} seconds after client startup (last helper head slot: {}, justified={}, finalized={})",
                minimum_source_head_slot,
                minimum_source_finalized_slot,
                HEAD_SYNC_TIMEOUT_SECS,
                fork_choice_head_slot(&source),
                source.justified.slot,
                source.finalized.slot,
            )));
        }

        if let Some(err) = last_client_error {
            return Err(HeadSyncWaitError::RetryableClient(format!(
                "Client under test fork_choice endpoint never became readable within {} seconds while comparing against the local LeanSpec helper head: {}",
                HEAD_SYNC_TIMEOUT_SECS, err
            )));
        }

        return Err(HeadSyncWaitError::RetryableClient(format!(
            "Client under test could not be compared with the local LeanSpec helper head within {} seconds",
            HEAD_SYNC_TIMEOUT_SECS
        )));
    };
    Err(HeadSyncWaitError::RetryableClient(format!(
        "Client under test never reached required head slot {} and caught up to the local LeanSpec helper head slot within {} seconds (allowed lag: {} slots, helper-before head: {:#x} at slot {} [justified={}, finalized={}], client head: {:#x} at slot {} [justified={}, finalized={}], helper-after head: {:#x} at slot {} [justified={}, finalized={}])",
        minimum_client_head_slot,
        HEAD_SYNC_TIMEOUT_SECS,
        HEAD_SYNC_MAX_SLOT_LAG,
        observation.source_before_head,
        observation.source_before_head_slot,
        observation.source_before_justified_slot,
        observation.source_before_finalized_slot,
        observation.client_head,
        observation.client_head_slot,
        observation.client_justified_slot,
        observation.client_finalized_slot,
        observation.source_after_head,
        observation.source_after_head_slot,
        observation.source_after_justified_slot,
        observation.source_after_finalized_slot,
    )))
}

async fn wait_for_client_to_reach_source_head(
    context: &mut PostGenesisSyncContext,
    minimum_source_head_slot: u64,
    minimum_source_finalized_slot: u64,
    minimum_client_head_slot: u64,
) -> (ForkChoiceResponse, ForkChoiceResponse) {
    try_wait_for_client_to_reach_source_head(
        context,
        minimum_source_head_slot,
        minimum_source_finalized_slot,
        minimum_client_head_slot,
    )
    .await
    .unwrap_or_else(|err| panic!("{}", err.into_message()))
}

async fn wait_for_client_to_reach_source_head_with_client_restart(
    test: &Test,
    context: &mut PostGenesisSyncContext,
    minimum_source_head_slot: u64,
    minimum_source_finalized_slot: u64,
    minimum_client_head_slot: u64,
    phase: &str,
) -> (ForkChoiceResponse, ForkChoiceResponse) {
    match try_wait_for_client_to_reach_source_head(
        context,
        minimum_source_head_slot,
        minimum_source_finalized_slot,
        minimum_client_head_slot,
    )
    .await
    {
        Ok(result) => result,
        Err(HeadSyncWaitError::NonRetryable(message)) => panic!("{message}"),
        Err(HeadSyncWaitError::RetryableClient(first_error)) => {
            let client_name = context.client_under_test.kind.clone();
            eprintln!(
                "Restarting client under test {client_name} after it failed to catch up during {phase}: {first_error}"
            );
            context
                .restart_client_under_test(test)
                .await
                .unwrap_or_else(|restart_err| {
                    panic!(
                        "Unable to restart client under test {client_name} after failed {phase}: {restart_err}. Initial catch-up failure: {first_error}"
                    )
                });

            try_wait_for_client_to_reach_source_head(
                context,
                minimum_source_head_slot,
                minimum_source_finalized_slot,
                minimum_client_head_slot,
            )
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Client under test {client_name} still failed to catch up during {phase} after one restart: {}. Initial catch-up failure before restart: {first_error}",
                    err.into_message()
                )
            })
        }
    }
}

async fn wait_for_helper_finalized_above_client_head(
    context: &mut PostGenesisSyncContext,
    client_head_slot: u64,
) -> Result<ForkChoiceResponse, String> {
    let mut last_fork_choice = None;
    let client_name = context.client_under_test.kind.clone();
    let deadline =
        Instant::now() + Duration::from_secs(HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS);

    while Instant::now() < deadline {
        match context.try_load_live_helper_fork_choice().await {
            Ok(fork_choice) => {
                if fork_choice.finalized.slot > client_head_slot {
                    return Ok(fork_choice);
                }
                last_fork_choice = Some(fork_choice);
            }
            Err(err) => {
                return Err(helper_failed_after_test_start_message(
                    &client_name,
                    "waiting for helper finalization to pass the paused client head",
                    &err,
                ));
            }
        }

        sleep(Duration::from_secs(1)).await;
    }

    match last_fork_choice {
        Some(fork_choice) => Err(format!(
            "Local LeanSpec helpers never finalized above paused client head slot {} within {} seconds (last helper head: {:#x} at slot {}, justified={}, finalized={}).",
            client_head_slot,
            HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS,
            fork_choice.head,
            fork_choice_head_slot(&fork_choice),
            fork_choice.justified.slot,
            fork_choice.finalized.slot,
        )),
        None => Err(format!(
            "Local LeanSpec helpers never produced a forkchoice response while waiting {} seconds for finalization above paused client head slot {}.",
            HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS,
            client_head_slot,
        )),
    }
}

async fn wait_for_honest_helper_finalized_agreement(
    context: &mut PostGenesisSyncContext,
    minimum_finalized_slot: u64,
    phase: &str,
) -> ForkChoiceResponse {
    let mut last_error = None;
    let deadline =
        Instant::now() + Duration::from_secs(HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS);

    while Instant::now() < deadline {
        match context
            .try_load_agreed_helper_fork_choice(minimum_finalized_slot)
            .await
        {
            Ok(fork_choice) => return fork_choice,
            Err(err) => last_error = Some(err),
        }

        sleep(Duration::from_secs(1)).await;
    }

    panic!(
        "Honest LeanSpec helpers did not agree on a finalized checkpoint at or above slot {} within {} seconds while {}. Last observation: {}",
        minimum_finalized_slot,
        HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS,
        phase,
        last_error.unwrap_or_else(|| "no helper forkchoice responses were observed".to_string())
    );
}

async fn wait_for_helper_justified_above_client_head(
    context: &mut PostGenesisSyncContext,
    client_head_slot: u64,
) -> Result<ForkChoiceResponse, String> {
    let mut last_fork_choice = None;
    let client_name = context.client_under_test.kind.clone();
    let deadline =
        Instant::now() + Duration::from_secs(HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS);

    while Instant::now() < deadline {
        match context.try_load_live_helper_fork_choice().await {
            Ok(fork_choice) => {
                if fork_choice.justified.slot > client_head_slot {
                    return Ok(fork_choice);
                }
                last_fork_choice = Some(fork_choice);
            }
            Err(err) => {
                return Err(helper_failed_after_test_start_message(
                    &client_name,
                    "waiting for helper justification to pass the paused client head",
                    &err,
                ));
            }
        }

        sleep(Duration::from_secs(1)).await;
    }

    match last_fork_choice {
        Some(fork_choice) => Err(format!(
            "Local LeanSpec helpers never justified above paused client head slot {} within {} seconds (last helper head: {:#x} at slot {}, justified={}, finalized={}).",
            client_head_slot,
            HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS,
            fork_choice.head,
            fork_choice_head_slot(&fork_choice),
            fork_choice.justified.slot,
            fork_choice.finalized.slot,
        )),
        None => Err(format!(
            "Local LeanSpec helpers never produced a forkchoice response while waiting {} seconds for justification above paused client head slot {}.",
            HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS,
            client_head_slot,
        )),
    }
}

async fn wait_for_bad_peer_finalized_ahead(
    bad_peer: &mut RunningBadCheckpointPeer,
    source: &ForkChoiceResponse,
) -> ForkChoiceResponse {
    let source_finalized_slot = source.finalized.slot;
    let deadline = Instant::now() + Duration::from_secs(BAD_CHECKPOINT_PEER_AHEAD_TIMEOUT_SECS);
    let mut last_bad_fork_choice = None;
    let mut last_error = None;

    while Instant::now() < deadline {
        let bad_fork_choice = match bad_peer.try_load_fork_choice().await {
            Ok(fork_choice) => fork_choice,
            Err(err) => {
                match bad_peer.restart_after_retryable_exit(&err).await {
                    Ok(true) => {
                        last_error = Some(err);
                        sleep(Duration::from_secs(1)).await;
                        continue;
                    }
                    Ok(false) => {
                        last_error = Some(err);
                    }
                    Err(restart_err) => {
                        last_error = Some(format!(
                            "{err}; failed to restart adversarial helper: {restart_err}"
                        ));
                    }
                }
                sleep(Duration::from_secs(1)).await;
                continue;
            }
        };

        if bad_fork_choice.finalized.slot
            >= source_finalized_slot.saturating_add(BAD_CHECKPOINT_PEER_MIN_SLOT_LEAD)
        {
            return bad_fork_choice;
        }

        last_bad_fork_choice = Some(bad_fork_choice);
        sleep(Duration::from_secs(1)).await;
    }

    match last_bad_fork_choice {
        Some(last) => {
            let last_error = last_error
                .as_deref()
                .unwrap_or("no adversarial peer forkchoice request errors observed");
            panic!(
                "Adversarial peer never stayed at least {} finalized slots ahead of the honest helper network within {} seconds (honest finalized slot: {}, adversarial finalized slot: {}, adversarial head slot: {}, last error: {})",
                BAD_CHECKPOINT_PEER_MIN_SLOT_LEAD,
                BAD_CHECKPOINT_PEER_AHEAD_TIMEOUT_SECS,
                source_finalized_slot,
                last.finalized.slot,
                fork_choice_head_slot(&last),
                last_error,
            );
        }
        None => {
            let last_error = last_error
                .as_deref()
                .unwrap_or("no adversarial peer forkchoice request attempted");
            panic!(
                "Adversarial peer never produced a forkchoice response while waiting {} seconds for it to stay at least {} finalized slots ahead of the honest helper network (honest finalized slot: {}, last error: {})",
                BAD_CHECKPOINT_PEER_AHEAD_TIMEOUT_SECS,
                BAD_CHECKPOINT_PEER_MIN_SLOT_LEAD,
                source_finalized_slot,
                last_error,
            );
        }
    }
}

async fn wait_for_bad_peer_fork_choice(
    bad_peer: &mut RunningBadCheckpointPeer,
    phase: &str,
) -> ForkChoiceResponse {
    let deadline = Instant::now() + Duration::from_secs(BAD_CHECKPOINT_PEER_AHEAD_TIMEOUT_SECS);
    let mut last_error = None;

    while Instant::now() < deadline {
        match bad_peer.try_load_fork_choice().await {
            Ok(fork_choice) => return fork_choice,
            Err(err) => last_error = Some(err),
        }

        sleep(Duration::from_secs(1)).await;
    }

    let last_error = last_error
        .as_deref()
        .unwrap_or("no adversarial peer forkchoice request attempted");
    panic!(
        "Unable to load adversarial peer forkchoice {phase} within {} seconds: {}",
        BAD_CHECKPOINT_PEER_AHEAD_TIMEOUT_SECS, last_error,
    );
}

fn assert_client_rejected_bad_checkpoint(
    phase: &str,
    source: &ForkChoiceResponse,
    client: &ForkChoiceResponse,
    bad_peer: &ForkChoiceResponse,
) {
    let source_head_slot = fork_choice_head_slot(source);
    let client_head_slot = fork_choice_head_slot(client);
    let bad_head_slot = fork_choice_head_slot(bad_peer);

    if bad_peer.finalized.slot
        >= source
            .finalized
            .slot
            .saturating_add(BAD_CHECKPOINT_PEER_MIN_SLOT_LEAD)
    {
        assert_ne!(
            (client.finalized.root, client.finalized.slot),
            (bad_peer.finalized.root, bad_peer.finalized.slot),
            "{phase}: client finalized the adversarial peer's bad checkpoint at slot {} instead of following the honest helper network finalized slot {}",
            bad_peer.finalized.slot,
            source.finalized.slot,
        );
    }

    if bad_head_slot >= source_head_slot.saturating_add(BAD_CHECKPOINT_PEER_MIN_SLOT_LEAD) {
        assert!(
            !head_slot_is_caught_up(client_head_slot, bad_head_slot),
            "{phase}: client tracked the adversarial peer head slot {} while the honest helper network head was at slot {} (client head slot: {}, allowed lag: {})",
            bad_head_slot,
            source_head_slot,
            client_head_slot,
            HEAD_SYNC_MAX_SLOT_LAG,
        );
    }
}

dyn_async! {
    pub async fn run_sync_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        for client in &clients {
            let checkpoint_sync_genesis_time = default_genesis_time();
            let helper_fork_digest_profile = if selected_lean_devnet().uses_latest_leanspec_format() {
                HelperGossipForkDigestProfile::SelectedDevnet
            } else {
                HelperGossipForkDigestProfile::LegacyDevnet0
            };
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "sync: checkpoint sync fresh start".to_string(),
                    description: "Starts a local LeanSpec helper mesh, checkpoint-syncs the validator client under test from a finalized checkpoint, connects it to the helper mesh, and checks that the client catches up to the helper's live head slot.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: Duration::from_secs(CHECKPOINT_SYNC_FRESH_START_OUTER_TIMEOUT_SECS),
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: checkpoint_sync_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: Some(
                            LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0.to_string(),
                        ),
                        split_helper_validators_across_mesh: false,
                        helper_peer_count: SYNC_HELPER_PEER_COUNT,
                        helper_fork_digest_profile,
                    },
                },
                test_checkpoint_sync_fresh_start,
            )
            .await;

            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "sync: late joiner syncs and finalizes via LeanSpec helper mesh".to_string(),
                    description: "Starts a local LeanSpec helper mesh, waits for the helpers to finalize, starts the validator client under test late from the helper checkpoint, and checks that it catches up and finalizes close to the helper source.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: Duration::from_secs(LATE_JOINER_HELPER_MESH_OUTER_TIMEOUT_SECS),
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: default_genesis_time(),
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: Some(
                            LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0.to_string(),
                        ),
                        split_helper_validators_across_mesh: false,
                        helper_peer_count: SYNC_HELPER_PEER_COUNT,
                        helper_fork_digest_profile,
                    },
                },
                test_sync_late_joiner_syncs_and_finalizes,
            )
            .await;

            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "sync: late joiner syncs and finalizes".to_string(),
                    description: "Starts two validator instances of the selected client from genesis, starts a third same-client observer instance late using the first two instances as bootnodes, and checks that the late joiner catches up and finalizes close to the bootstrap mesh.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: Duration::from_secs(LATE_JOINER_SAME_CLIENT_OUTER_TIMEOUT_SECS),
                    test_data: SameClientLateJoinerTestData {
                        client_under_test: client.clone(),
                        genesis_time: default_genesis_time(),
                    },
                },
                test_sync_late_joiner_syncs_and_finalizes_via_same_client_mesh,
            )
            .await;

            let head_behind_finalized_recovery_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "sync: head behind finalized recovery".to_string(),
                    description: "Starts the client under test alongside a local LeanSpec helper mesh, pauses it after the helper source has finalized and the client is caught up, lets the helpers finalize beyond the paused head, unpauses it, and checks that it catches back up to the helper live head slot.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: Duration::from_secs(HEAD_BEHIND_FINALIZED_RECOVERY_OUTER_TIMEOUT_SECS),
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: head_behind_finalized_recovery_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: false,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: Some(
                            LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0.to_string(),
                        ),
                        split_helper_validators_across_mesh: false,
                        helper_peer_count: SYNC_HELPER_PEER_COUNT,
                        helper_fork_digest_profile,
                    },
                },
                test_sync_head_behind_finalized_recovery,
            )
            .await;

            let bad_checkpoint_rejection_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "sync: head behind finalized rejects ahead bad checkpoint".to_string(),
                    description: "Starts the client under test alongside two honest local LeanSpec helpers and an isolated adversarial LeanSpec peer whose validly signed chain is artificially ahead, pauses the client until the honest network finalizes beyond its head, unpauses it, and checks that the client follows the agreeing honest peers instead of the bad checkpoint from the single ahead peer.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: Duration::from_secs(BAD_CHECKPOINT_REJECTION_OUTER_TIMEOUT_SECS),
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: bad_checkpoint_rejection_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: false,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: Some(
                            LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0.to_string(),
                        ),
                        split_helper_validators_across_mesh: false,
                        helper_peer_count: SYNC_HELPER_PEER_COUNT,
                        helper_fork_digest_profile,
                    },
                },
                test_sync_head_behind_finalized_rejects_ahead_bad_checkpoint,
            )
            .await;

            let head_recovery_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "sync: head recovery".to_string(),
                    description: "Starts the client under test alongside a local LeanSpec helper mesh, pauses it after the helper source has finalized and the client is caught up, lets the helpers justify beyond the paused head, unpauses it, and checks that it catches back up to the helper live head slot.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: Duration::from_secs(HEAD_RECOVERY_OUTER_TIMEOUT_SECS),
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: head_recovery_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: false,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: Some(
                            LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0.to_string(),
                        ),
                        split_helper_validators_across_mesh: false,
                        helper_peer_count: SYNC_HELPER_PEER_COUNT,
                        helper_fork_digest_profile,
                    },
                },
                test_sync_head_recovery,
            )
            .await;
        }
    }
}

dyn_async! {
    async fn test_sync_late_joiner_syncs_and_finalizes<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let client_name = test_data.client_under_test.name.clone();

        let helper_mesh = timeout(
            Duration::from_secs(CHECKPOINT_SYNC_HELPER_MESH_TIMEOUT_SECS),
            start_checkpoint_sync_helper_mesh(&test_data),
        )
        .await
        .unwrap_or_else(|_| {
            panic!(
                "late-joiner checkpoint helper mesh setup for {client_name} exceeded {CHECKPOINT_SYNC_HELPER_MESH_TIMEOUT_SECS} seconds before client startup"
            )
        });

        let mut context = timeout(
            Duration::from_secs(CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS),
            start_checkpoint_sync_client_context(test, &test_data, helper_mesh),
        )
        .await
        .unwrap_or_else(|_| {
            panic!(
                "late-joiner checkpoint client {client_name} did not start within {CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS} seconds after helper checkpoint readiness"
            )
        });

        assert!(
            context.source_fork_choice.finalized.slot > 0,
            "helper source should expose a non-genesis finalized checkpoint before starting late-joiner client {client_name}"
        );

        let checkpoint_sync_boundary = context.source_fork_choice.finalized.slot;

        wait_for_client_to_reach_source_head_with_client_restart(
            test,
            &mut context,
            checkpoint_sync_boundary,
            checkpoint_sync_boundary,
            0,
            "late-joiner checkpoint catch-up",
        )
        .await;

        wait_for_late_joiner_client_to_finalize_near_source(&mut context).await;
    }
}

dyn_async! {
    async fn test_sync_late_joiner_syncs_and_finalizes_via_same_client_mesh<'a>(test: &'a mut Test, test_data: SameClientLateJoinerTestData) {
        let client_name = test_data.client_under_test.name.clone();

        let mut running_clients = Vec::with_capacity(SAME_CLIENT_GENESIS_NODE_COUNT);
        for node_index in 0..SAME_CLIENT_GENESIS_BOOTSTRAP_PEER_COUNT {
            let bootnodes = running_clients
                .iter()
                .map(|node: &RunningSameClientGenesisNode| node.bootnode.clone())
                .collect::<Vec<_>>();
            let running_client = timeout(
                Duration::from_secs(CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS),
                start_same_client_genesis_node(test, &test_data, node_index, bootnodes),
            )
            .await
            .unwrap_or_else(|_| {
                panic!(
                    "same-client genesis bootstrap node {node_index} for {client_name} did not start within {CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS} seconds"
                )
            });
            running_clients.push(running_client);
        }

        sleep_until_unix_time(
            test_data.genesis_time + SAME_CLIENT_GENESIS_LATE_START_AFTER_GENESIS_SECS,
        )
        .await;

        let late_joiner_bootnodes = running_clients
            .iter()
            .map(|node| node.bootnode.clone())
            .collect::<Vec<_>>();
        let late_joiner = timeout(
            Duration::from_secs(CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS),
            start_same_client_genesis_node(
                test,
                &test_data,
                SAME_CLIENT_GENESIS_LATE_JOINER_INDEX,
                late_joiner_bootnodes,
            ),
        )
        .await
        .unwrap_or_else(|_| {
            panic!(
                "same-client genesis late joiner for {client_name} did not start within {CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS} seconds"
            )
        });
        running_clients.push(late_joiner);

        wait_for_same_client_late_joiner_to_finalize(
            &running_clients[SAME_CLIENT_GENESIS_VALIDATOR_NODE_INDEX],
            &running_clients[SAME_CLIENT_GENESIS_LATE_JOINER_INDEX],
        )
        .await;
    }
}

async fn start_same_client_genesis_node(
    test: &Test,
    test_data: &SameClientLateJoinerTestData,
    node_index: usize,
    bootnodes: Vec<String>,
) -> RunningSameClientGenesisNode {
    let client_name = &test_data.client_under_test.name;
    let client_kind = lean_client_kind(client_name)
        .unwrap_or_else(|err| panic!("Unable to build same-client genesis node: {err}"));
    let node_id = format!("{client_kind}_{node_index}");
    let private_key = crate::utils::libp2p_mock::deterministic_client_private_key_hex_for_node(
        client_name,
        &node_id,
    );
    let environment = same_client_genesis_environment(
        &node_id,
        &private_key,
        test_data.genesis_time,
        SAME_CLIENT_GENESIS_VALIDATOR_COUNT,
        node_index,
        &bootnodes,
    );
    let client = start_client_under_test_with_retry(
        test,
        test_data.client_under_test.name.clone(),
        environment,
    )
    .await;
    let metadata = bootnode_metadata_for_client(&private_key, client.ip, LEAN_CLIENT_P2P_PORT);
    let bootnode = lean_bootnodes_for_client(client_kind, &[metadata]);

    RunningSameClientGenesisNode {
        node_id,
        client,
        bootnode,
    }
}

fn same_client_genesis_environment(
    node_id: &str,
    private_key: &str,
    genesis_time: u64,
    genesis_validator_count: usize,
    node_index: usize,
    bootnodes: &[String],
) -> HashMap<String, String> {
    let mut environment = lean_environment();
    let validator_indices = same_client_genesis_validator_indices(node_index);
    environment.insert(
        BOOTNODES_ENVIRONMENT_VARIABLE.to_string(),
        if bootnodes.is_empty() {
            "none".to_string()
        } else {
            bootnodes.join(",")
        },
    );
    environment.insert(
        CLIENT_PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(),
        private_key.to_string(),
    );
    environment.insert(
        LEAN_ATTESTATION_COMMITTEE_COUNT_ENVIRONMENT_VARIABLE.to_string(),
        SAME_CLIENT_GENESIS_ATTESTATION_COMMITTEE_COUNT.to_string(),
    );
    environment.insert(
        LEAN_GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE.to_string(),
        genesis_validator_count.to_string(),
    );
    environment.insert(
        LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE.to_string(),
        genesis_time.to_string(),
    );
    environment.insert(
        NODE_ID_ENVIRONMENT_VARIABLE.to_string(),
        node_id.to_string(),
    );

    if let Some(validator_indices) = validator_indices {
        let first_validator_index = validator_indices
            .split(',')
            .next()
            .expect("validator index set must not be empty");
        environment.insert(
            LEAN_VALIDATOR_INDEX_ENVIRONMENT_VARIABLE.to_string(),
            first_validator_index.to_string(),
        );
        environment.insert(
            LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE.to_string(),
            validator_indices,
        );
        environment.insert(
            IS_AGGREGATOR_ENVIRONMENT_VARIABLE.to_string(),
            "1".to_string(),
        );
    } else {
        environment.insert(
            LEAN_CLIENT_RUNTIME_ROLE_ENVIRONMENT_VARIABLE.to_string(),
            "observer".to_string(),
        );
    }

    environment
}

fn same_client_genesis_validator_indices(node_index: usize) -> Option<String> {
    match node_index {
        0 => Some("0,1".to_string()),
        1 => Some("2".to_string()),
        _ => None,
    }
}

async fn sleep_until_unix_time(target_unix_time: u64) {
    loop {
        let now = current_unix_time();
        if now >= target_unix_time {
            return;
        }
        sleep(Duration::from_secs((target_unix_time - now).min(1))).await;
    }
}

async fn wait_for_same_client_late_joiner_to_finalize(
    source: &RunningSameClientGenesisNode,
    late_joiner: &RunningSameClientGenesisNode,
) {
    let client_name = late_joiner.client.kind.clone();
    let deadline = Instant::now() + Duration::from_secs(LATE_JOINER_FINALIZATION_TIMEOUT_SECS);
    let mut last_observation = None;

    while Instant::now() < deadline {
        let source_result = try_load_fork_choice_response(&source.client).await;
        let client_result = try_load_fork_choice_response(&late_joiner.client).await;

        let observation = LateJoinerObservation {
            source: source_result.as_ref().ok().cloned(),
            client: client_result.as_ref().ok().cloned(),
            source_error: source_result.as_ref().err().cloned(),
            client_error: client_result.as_ref().err().cloned(),
        };

        if let (Ok(source_fork_choice), Ok(client_fork_choice)) = (&source_result, &client_result) {
            let head_slot_delta = fork_choice_head_slot(source_fork_choice)
                .abs_diff(fork_choice_head_slot(client_fork_choice));
            let finalized_slot_lag = source_fork_choice
                .finalized
                .slot
                .saturating_sub(client_fork_choice.finalized.slot);

            if client_fork_choice.finalized.slot > 0
                && source_fork_choice.finalized.slot > 0
                && head_slot_delta <= HEAD_SYNC_MAX_SLOT_LAG
                && finalized_slot_lag <= LATE_JOINER_MAX_FINALIZED_SLOT_LAG
            {
                return;
            }
        }

        last_observation = Some(observation);
        sleep(Duration::from_secs(LATE_JOINER_POLL_INTERVAL_SECS)).await;
    }

    panic!(
        "same-client genesis late joiner {} ({}) did not finalize close to bootstrap node {} within {} seconds. Required: both finalized slot > 0, head delta <= {} slots, finalized slot lag <= {}. Last observation: {}",
        late_joiner.node_id,
        client_name,
        source.node_id,
        LATE_JOINER_FINALIZATION_TIMEOUT_SECS,
        HEAD_SYNC_MAX_SLOT_LAG,
        LATE_JOINER_MAX_FINALIZED_SLOT_LAG,
        last_observation
            .as_ref()
            .map(format_late_joiner_observation)
            .unwrap_or_else(|| "no forkchoice observations collected".to_string()),
    );
}

async fn wait_for_late_joiner_client_to_finalize_near_source(context: &mut PostGenesisSyncContext) {
    let client_name = context.client_under_test.kind.clone();
    let deadline = Instant::now() + Duration::from_secs(LATE_JOINER_FINALIZATION_TIMEOUT_SECS);
    let mut last_observation = None;

    while Instant::now() < deadline {
        let source_result = context.try_load_live_helper_fork_choice().await;
        let client_result = try_load_fork_choice_response(&context.client_under_test).await;

        let observation = LateJoinerObservation {
            source: source_result.as_ref().ok().cloned(),
            client: client_result.as_ref().ok().cloned(),
            source_error: source_result.as_ref().err().cloned(),
            client_error: client_result.as_ref().err().cloned(),
        };

        if let (Ok(source_fork_choice), Ok(client_fork_choice)) = (&source_result, &client_result) {
            let finalized_slot_lag = source_fork_choice
                .finalized
                .slot
                .saturating_sub(client_fork_choice.finalized.slot);

            if client_fork_choice.finalized.slot > 0
                && client_caught_up_to_source(source_fork_choice, client_fork_choice)
                && finalized_slot_lag <= LATE_JOINER_MAX_FINALIZED_SLOT_LAG
            {
                return;
            }
        }

        last_observation = Some(observation);
        sleep(Duration::from_secs(LATE_JOINER_POLL_INTERVAL_SECS)).await;
    }

    panic!(
        "sync late-joiner client {client_name} did not finalize close to the local LeanSpec helper source within {} seconds after catching up. Required: client finalized slot > 0, client head within {} slots of helper source, finalized slot lag <= {}. Last observation: {}",
        LATE_JOINER_FINALIZATION_TIMEOUT_SECS,
        HEAD_SYNC_MAX_SLOT_LAG,
        LATE_JOINER_MAX_FINALIZED_SLOT_LAG,
        last_observation
            .as_ref()
            .map(format_late_joiner_observation)
            .unwrap_or_else(|| "no forkchoice observations collected".to_string()),
    );
}

fn format_late_joiner_observation(observation: &LateJoinerObservation) -> String {
    let source = observation
        .source
        .as_ref()
        .map(format_late_joiner_fork_choice)
        .unwrap_or_else(|| {
            format!(
                "error={}",
                observation
                    .source_error
                    .as_deref()
                    .unwrap_or("unknown source forkchoice error")
            )
        });
    let client = observation
        .client
        .as_ref()
        .map(format_late_joiner_fork_choice)
        .unwrap_or_else(|| {
            format!(
                "error={}",
                observation
                    .client_error
                    .as_deref()
                    .unwrap_or("unknown client forkchoice error")
            )
        });

    format!("source {{{source}}}; client {{{client}}}")
}

fn format_late_joiner_fork_choice(fork_choice: &ForkChoiceResponse) -> String {
    format!(
        "head={:#x} head_slot={} justified={:#x}:{} finalized={:#x}:{}",
        fork_choice.head,
        fork_choice_head_slot(fork_choice),
        fork_choice.justified.root,
        fork_choice.justified.slot,
        fork_choice.finalized.root,
        fork_choice.finalized.slot,
    )
}

dyn_async! {
    async fn test_checkpoint_sync_fresh_start<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let client_name = test_data.client_under_test.name.clone();

        let helper_mesh = timeout(
            Duration::from_secs(CHECKPOINT_SYNC_HELPER_MESH_TIMEOUT_SECS),
            start_checkpoint_sync_helper_mesh(&test_data),
        )
        .await
        .unwrap_or_else(|_| {
            panic!(
                "checkpoint-sync helper mesh setup for {client_name} exceeded {CHECKPOINT_SYNC_HELPER_MESH_TIMEOUT_SECS} seconds before client startup"
            )
        });

        let (checkpoint_sync_boundary, source_fork_choice, client_fork_choice) = timeout(
            Duration::from_secs(CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS),
            async {
                let mut context =
                    start_checkpoint_sync_client_context(test, &test_data, helper_mesh).await;
                assert!(
                    context.source_fork_choice.finalized.slot > 0,
                    "helper source should expose a non-genesis finalized checkpoint before checkpoint-syncing the client under test"
                );
                let checkpoint_sync_boundary = context.source_fork_choice.finalized.slot;

                let (source_fork_choice, client_fork_choice) =
                    wait_for_client_to_reach_source_head(
                        &mut context,
                        checkpoint_sync_boundary,
                        checkpoint_sync_boundary,
                        0,
                    )
                    .await;
                (
                    checkpoint_sync_boundary,
                    source_fork_choice,
                    client_fork_choice,
                )
            },
        )
        .await
        .unwrap_or_else(|_| {
            panic!(
                "checkpoint-sync client {client_name} did not start and catch up to the helper head within {CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS} seconds after checkpoint sync startup"
            )
        });

        let source_head_slot = fork_choice_head_slot(&source_fork_choice);
        let client_head_slot = if client_fork_choice.head == source_fork_choice.head {
            source_head_slot
        } else {
            fork_choice_head_slot(&client_fork_choice)
        };

        assert!(
            source_head_slot >= checkpoint_sync_boundary,
            "helper head should stay at or ahead of the finalized checkpoint used for checkpoint sync"
        );
        assert!(
            client_head_slot >= checkpoint_sync_boundary,
            "checkpoint-synced client should advance beyond the finalized checkpoint boundary while catching up to the helper head"
        );
        assert!(
            client_caught_up_to_source(&source_fork_choice, &client_fork_choice),
            "checkpoint-synced client should catch up to the helper live head slot (client slot: {}, helper slot: {}, allowed lag: {} slots)",
            client_head_slot,
            source_head_slot,
            HEAD_SYNC_MAX_SLOT_LAG
        );
    }
}

dyn_async! {
    async fn test_sync_head_recovery<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let client_name = test_data.client_under_test.name.clone();

        let mut context = timeout(
            Duration::from_secs(HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS),
            start_post_genesis_sync_context(test, &test_data),
        )
        .await
        .unwrap_or_else(|_| {
            panic!(
                "head-recovery helper mesh and client startup for {} exceeded {} seconds",
                client_name, HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS
            )
        });

        let (source_before_pause, client_before_pause) =
            wait_for_client_to_reach_source_head_with_client_restart(
                test,
                &mut context,
                0,
                1,
                PRE_PAUSE_MIN_CLIENT_HEAD_SLOT,
                "pre-pause head-recovery catch-up",
            )
            .await;
        let paused_client_head_slot = fork_choice_head_slot(&client_before_pause);
        let paused_client_head = client_before_pause.head;

        assert!(
            source_before_pause.finalized.slot > 0,
            "helper source should reach a non-genesis finalized checkpoint before pausing the client under test"
        );
        if let Err(err) = context.client_under_test.pause().await {
            let error = err.to_string();
            panic!(
                "Unable to pause client under test {} after reading head {:#x} at slot {}: {}",
                client_name, paused_client_head, paused_client_head_slot, error
            );
        }

        let helper_justified_above_head =
            wait_for_helper_justified_above_client_head(&mut context, paused_client_head_slot).await;

        if let Err(err) = context.client_under_test.unpause().await {
            let error = err.to_string();
            panic!(
                "Unable to unpause client under test {} after pausing at head {:#x} slot {}: {}",
                client_name, paused_client_head, paused_client_head_slot, error
            );
        }

        let helper_justified_above_head =
            helper_justified_above_head.unwrap_or_else(|err| panic!("{err}"));
        let recovery_justified_boundary = helper_justified_above_head.justified.slot;

        assert!(
            recovery_justified_boundary > paused_client_head_slot,
            "helper justified slot ({}) should advance beyond paused client head slot ({}) before recovery",
            recovery_justified_boundary,
            paused_client_head_slot
        );

        let (source_after_recovery, client_after_recovery) =
            wait_for_client_to_reach_source_head(&mut context, 0, 0, 0).await;
        let source_head_slot = fork_choice_head_slot(&source_after_recovery);
        let client_head_slot = fork_choice_head_slot(&client_after_recovery);

        assert!(
            client_head_slot.saturating_add(HEAD_SYNC_MAX_SLOT_LAG) >= recovery_justified_boundary,
            "unpaused client should advance within the allowed lag of the helper justified boundary that passed its paused head (client slot: {}, helper justified boundary: {}, paused client slot: {}, allowed lag: {})",
            client_head_slot,
            recovery_justified_boundary,
            paused_client_head_slot,
            HEAD_SYNC_MAX_SLOT_LAG
        );
        assert!(
            client_caught_up_to_source(&source_after_recovery, &client_after_recovery),
            "unpaused client should catch up to the helper live head slot (client slot: {}, helper slot: {}, allowed lag: {} slots)",
            client_head_slot,
            source_head_slot,
            HEAD_SYNC_MAX_SLOT_LAG
        );
    }
}

dyn_async! {
    async fn test_sync_head_behind_finalized_recovery<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let client_name = test_data.client_under_test.name.clone();

        let mut context = timeout(
            Duration::from_secs(HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS),
            start_post_genesis_sync_context(test, &test_data),
        )
        .await
        .unwrap_or_else(|_| {
            panic!(
                "head-behind-finalized helper mesh and client startup for {} exceeded {} seconds",
                client_name, HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS
            )
        });

        let (source_before_pause, client_before_pause) =
            wait_for_client_to_reach_source_head_with_client_restart(
                test,
                &mut context,
                0,
                1,
                PRE_PAUSE_MIN_CLIENT_HEAD_SLOT,
                "pre-pause finalized-recovery catch-up",
            )
            .await;
        let paused_client_head_slot = fork_choice_head_slot(&client_before_pause);
        let paused_client_head = client_before_pause.head;

        assert!(
            source_before_pause.finalized.slot > 0,
            "helper source should reach a non-genesis finalized checkpoint before pausing the client under test"
        );
        context
            .client_under_test
            .pause()
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Unable to pause client under test {} after reading head {:#x} at slot {}: {}",
                    client_name, paused_client_head, paused_client_head_slot, err
                )
            });

        let helper_finalized_above_head =
            wait_for_helper_finalized_above_client_head(&mut context, paused_client_head_slot).await;

        context
            .client_under_test
            .unpause()
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Unable to unpause client under test {} after pausing at head {:#x} slot {}: {}",
                    client_name, paused_client_head, paused_client_head_slot, err
                )
            });

        let helper_finalized_above_head =
            helper_finalized_above_head.unwrap_or_else(|err| panic!("{err}"));
        let recovery_finalized_boundary = helper_finalized_above_head.finalized.slot;

        assert!(
            recovery_finalized_boundary > paused_client_head_slot,
            "helper finalized slot ({}) should advance beyond paused client head slot ({}) before recovery",
            recovery_finalized_boundary,
            paused_client_head_slot
        );

        let (source_after_recovery, client_after_recovery) =
            wait_for_client_to_reach_source_head(&mut context, 0, 0, 0).await;
        let source_head_slot = fork_choice_head_slot(&source_after_recovery);
        let client_head_slot = fork_choice_head_slot(&client_after_recovery);

        assert!(
            client_head_slot.saturating_add(HEAD_SYNC_MAX_SLOT_LAG) >= recovery_finalized_boundary,
            "unpaused client should advance within the allowed lag of the helper finalized boundary that passed its paused head (client slot: {}, helper finalized boundary: {}, paused client slot: {}, allowed lag: {})",
            client_head_slot,
            recovery_finalized_boundary,
            paused_client_head_slot,
            HEAD_SYNC_MAX_SLOT_LAG
        );
        assert!(
            client_caught_up_to_source(&source_after_recovery, &client_after_recovery),
            "unpaused client should catch up to the helper live head slot (client slot: {}, helper slot: {}, allowed lag: {} slots)",
            client_head_slot,
            source_head_slot,
            HEAD_SYNC_MAX_SLOT_LAG
        );
    }
}

dyn_async! {
    async fn test_sync_head_behind_finalized_rejects_ahead_bad_checkpoint<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let client_name = test_data.client_under_test.name.clone();
        let mut bad_peer = start_bad_checkpoint_peer(&test_data).await;
        let bad_peer_bootnode = bad_peer.bootnode_for_client(&client_name);

        let mut context = timeout(
            Duration::from_secs(HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS),
            start_post_genesis_sync_context_with_extra_bootnodes_after_helper_agreement(
                test,
                &test_data,
                vec![bad_peer_bootnode],
            ),
        )
        .await
        .unwrap_or_else(|_| {
            panic!(
                "head-behind-finalized bad-checkpoint helper mesh and client startup for {} exceeded {} seconds",
                client_name, HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS
            )
        });

        let honest_helper_agreement_before_pause = wait_for_honest_helper_finalized_agreement(
            &mut context,
            1,
            "preparing the bad-checkpoint rejection test before pausing the client",
        )
        .await;
        let minimum_source_head_slot = fork_choice_head_slot(&honest_helper_agreement_before_pause);
        let (source_before_pause, client_before_pause) =
            wait_for_client_to_reach_source_head_with_client_restart(
                test,
                &mut context,
                minimum_source_head_slot,
                1,
                PRE_PAUSE_MIN_CLIENT_HEAD_SLOT,
                "pre-pause bad-checkpoint catch-up",
            )
            .await;
        let bad_before_pause =
            wait_for_bad_peer_finalized_ahead(&mut bad_peer, &source_before_pause).await;
        assert_client_rejected_bad_checkpoint(
            "before pause",
            &source_before_pause,
            &client_before_pause,
            &bad_before_pause,
        );

        let paused_client_head_slot = fork_choice_head_slot(&client_before_pause);
        let paused_client_head = client_before_pause.head;

        context
            .client_under_test
            .pause()
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Unable to pause client under test {} after reading head {:#x} at slot {}: {}",
                    client_name, paused_client_head, paused_client_head_slot, err
                )
            });

        let helper_finalized_above_head =
            wait_for_helper_finalized_above_client_head(&mut context, paused_client_head_slot)
                .await
                .unwrap_or_else(|err| {
                    panic!(
                        "Unable to observe an honest helper finalizing beyond paused client {} head {:#x} at slot {}: {}",
                        client_name, paused_client_head, paused_client_head_slot, err,
                    )
                });
        let bad_while_paused =
            wait_for_bad_peer_fork_choice(&mut bad_peer, "while client is paused").await;

        context
            .client_under_test
            .unpause()
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Unable to unpause client under test {} after pausing at head {:#x} slot {}: {}",
                    client_name, paused_client_head, paused_client_head_slot, err
                )
            });

        let recovery_finalized_boundary = helper_finalized_above_head.finalized.slot;

        assert!(
            recovery_finalized_boundary > paused_client_head_slot,
            "helper finalized slot ({}) should advance beyond paused client head slot ({}) before recovery",
            recovery_finalized_boundary,
            paused_client_head_slot
        );
        assert_client_rejected_bad_checkpoint(
            "while paused",
            &helper_finalized_above_head,
            &client_before_pause,
            &bad_while_paused,
        );

        let (source_after_recovery, client_after_recovery) =
            wait_for_client_to_reach_source_head(&mut context, 0, 0, 0).await;
        let bad_after_recovery =
            wait_for_bad_peer_finalized_ahead(&mut bad_peer, &source_after_recovery).await;
        let source_head_slot = fork_choice_head_slot(&source_after_recovery);
        let client_head_slot = fork_choice_head_slot(&client_after_recovery);

        assert!(
            client_head_slot.saturating_add(HEAD_SYNC_MAX_SLOT_LAG) >= recovery_finalized_boundary,
            "unpaused client should advance within the allowed lag of the helper finalized boundary that passed its paused head (client slot: {}, helper finalized boundary: {}, paused client slot: {}, allowed lag: {})",
            client_head_slot,
            recovery_finalized_boundary,
            paused_client_head_slot,
            HEAD_SYNC_MAX_SLOT_LAG
        );
        assert!(
            client_caught_up_to_source(&source_after_recovery, &client_after_recovery),
            "unpaused client should catch up to the honest helper live head slot (client slot: {}, helper slot: {}, allowed lag: {} slots)",
            client_head_slot,
            source_head_slot,
            HEAD_SYNC_MAX_SLOT_LAG
        );
        assert_client_rejected_bad_checkpoint(
            "after recovery",
            &source_after_recovery,
            &client_after_recovery,
            &bad_after_recovery,
        );
    }
}
