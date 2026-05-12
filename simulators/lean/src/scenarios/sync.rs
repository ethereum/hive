use crate::utils::helper::{
    start_checkpoint_sync_client_context, start_checkpoint_sync_helper_mesh,
    start_post_genesis_sync_context, HelperGossipForkDigestProfile, PostGenesisSyncContext,
    PostGenesisSyncTestData, LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0,
};
use crate::utils::util::{
    default_genesis_time, fork_choice_head_slot, lean_clients, load_fork_choice_response,
    run_data_test, selected_lean_devnet, ClientUnderTestRole, ForkChoiceResponse, LeanDevnet,
};
use alloy_primitives::B256;
use hivesim::{dyn_async, Client, Test};
use std::time::Duration;
use tokio::time::{sleep, timeout, Instant};

const CHECKPOINT_SYNC_HELPER_MESH_TIMEOUT_SECS: u64 = 420;
const CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS: u64 = 180;
const HEAD_SYNC_TIMEOUT_SECS: u64 = CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS;
const HEAD_BEHIND_FINALIZED_STARTUP_TIMEOUT_SECS: u64 = 600;
const HEAD_BEHIND_FINALIZED_HELPER_PROGRESS_TIMEOUT_SECS: u64 = 420;
const SYNC_HELPER_PEER_COUNT: usize = 2;
const HEAD_SYNC_MAX_SLOT_LAG: u64 = 2;

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

fn helper_failed_after_test_start_message(client_name: &str, phase: &str, error: &str) -> String {
    format!(
        "Local LeanSpec helper failed after client under test {client_name} was started while {phase}; test result is indeterminate and should not be interpreted as a client failure. Helper failure: {error}"
    )
}

async fn wait_for_client_to_reach_source_head(
    context: &mut PostGenesisSyncContext,
) -> (ForkChoiceResponse, ForkChoiceResponse) {
    let client_name = context.client_under_test.kind.clone();
    let mut last_observation = None;
    let deadline = Instant::now() + Duration::from_secs(HEAD_SYNC_TIMEOUT_SECS);

    while Instant::now() < deadline {
        let source_before_fork_choice = context
            .try_load_live_helper_fork_choice()
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "{}",
                    helper_failed_after_test_start_message(
                        &client_name,
                        "reading the helper live head before comparing client catch-up",
                        &err,
                    )
                )
            });
        let client_fork_choice = load_fork_choice_response(&context.client_under_test).await;
        if client_caught_up_to_source(&source_before_fork_choice, &client_fork_choice) {
            return (source_before_fork_choice, client_fork_choice);
        }

        let source_after_fork_choice = context
            .try_load_live_helper_fork_choice()
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "{}",
                    helper_failed_after_test_start_message(
                        &client_name,
                        "reading the helper live head after comparing client catch-up",
                        &err,
                    )
                )
            });
        last_observation = Some(capture_head_sync_observation(
            &source_before_fork_choice,
            &client_fork_choice,
            &source_after_fork_choice,
        ));
        if client_caught_up_to_source(&source_after_fork_choice, &client_fork_choice) {
            return (source_after_fork_choice, client_fork_choice);
        }

        sleep(Duration::from_secs(1)).await;
    }

    let Some(observation) = last_observation else {
        panic!(
            "Client under test could not be compared with the local LeanSpec helper head within {} seconds",
            HEAD_SYNC_TIMEOUT_SECS
        );
    };
    panic!(
        "Client under test never caught up to the local LeanSpec helper head slot within {} seconds (allowed lag: {} slots, helper-before head: {:#x} at slot {} [justified={}, finalized={}], client head: {:#x} at slot {} [justified={}, finalized={}], helper-after head: {:#x} at slot {} [justified={}, finalized={}])",
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
    );
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

dyn_async! {
    pub async fn run_sync_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        for client in &clients {
            let checkpoint_sync_genesis_time = default_genesis_time();
            let helper_fork_digest_profile = if selected_lean_devnet() == LeanDevnet::Devnet4 {
                HelperGossipForkDigestProfile::SelectedDevnet
            } else {
                HelperGossipForkDigestProfile::LegacyDevnet0
            };

            run_data_test(
                test,
                "sync: checkpoint sync fresh start".to_string(),
                "Starts a local LeanSpec helper mesh, checkpoint-syncs the validator client under test from a finalized checkpoint, connects it to the helper mesh, and checks that the client catches up to the helper's live head slot.".to_string(),
                false,
                PostGenesisSyncTestData {
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
                test_checkpoint_sync_fresh_start,
            )
            .await;

            let head_behind_finalized_recovery_genesis_time = default_genesis_time();
            run_data_test(
                test,
                "sync: head behind finalized recovery".to_string(),
                "Starts the client under test alongside a local LeanSpec helper mesh, pauses it after the helper source has finalized and the client is caught up, lets the helpers finalize beyond the paused head, unpauses it, and checks that it catches back up to the helper live head slot.".to_string(),
                false,
                PostGenesisSyncTestData {
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
                test_sync_head_behind_finalized_recovery,
            )
            .await;

            let head_recovery_genesis_time = default_genesis_time();
            run_data_test(
                test,
                "sync: head recovery".to_string(),
                "Starts the client under test alongside a local LeanSpec helper mesh, pauses it after the helper source has finalized and the client is caught up, lets the helpers justify beyond the paused head, unpauses it, and checks that it catches back up to the helper live head slot.".to_string(),
                false,
                PostGenesisSyncTestData {
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
                test_sync_head_recovery,
            )
            .await;
        }
    }
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

        let (source_fork_choice, client_fork_choice) = timeout(
            Duration::from_secs(CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS),
            async {
                let mut context =
                    start_checkpoint_sync_client_context(test, &test_data, helper_mesh).await;
                assert!(
                    context.source_fork_choice.finalized.slot > 0,
                    "helper source should expose a non-genesis finalized checkpoint before checkpoint-syncing the client under test"
                );

                wait_for_client_to_reach_source_head(&mut context).await
            },
        )
        .await
        .unwrap_or_else(|_| {
            panic!(
                "checkpoint-sync client {client_name} did not start and catch up to the helper head within {CHECKPOINT_SYNC_CLIENT_TO_HEAD_TIMEOUT_SECS} seconds after checkpoint sync startup"
            )
        });

        let checkpoint_sync_boundary = source_fork_choice.finalized.slot;
        let source_head_slot = fork_choice_head_slot(&source_fork_choice);
        let client_head_slot = fork_choice_head_slot(&client_fork_choice);

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
            wait_for_client_to_reach_source_head(&mut context).await;
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

        let helper_justified_above_head =
            wait_for_helper_justified_above_client_head(&mut context, paused_client_head_slot).await;

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
            wait_for_client_to_reach_source_head(&mut context).await;
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
            wait_for_client_to_reach_source_head(&mut context).await;
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
            wait_for_client_to_reach_source_head(&mut context).await;
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
