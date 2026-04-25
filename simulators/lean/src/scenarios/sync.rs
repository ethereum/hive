use crate::scenarios::helper::{
    default_genesis_time, fork_choice_head_slot, lean_clients, load_fork_choice_response,
    run_data_test, selected_lean_devnet, start_post_genesis_sync_context, ClientUnderTestRole,
    ForkChoiceResponse, HelperGossipForkDigestProfile, LeanDevnet, PostGenesisSyncContext,
    PostGenesisSyncTestData,
};
use alloy_primitives::B256;
use hivesim::{dyn_async, Client, Test};
use std::time::Duration;
use tokio::time::{sleep, timeout, Instant};

const HEAD_SYNC_TIMEOUT_SECS: u64 = 300;
const CHECKPOINT_SYNC_FRESH_START_TIMEOUT_SECS: u64 = 480;
const SYNC_HELPER_PEER_COUNT: usize = 3;

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

async fn wait_for_client_to_reach_source_head(
    context: &mut PostGenesisSyncContext,
) -> (ForkChoiceResponse, ForkChoiceResponse) {
    let mut last_observation = None;
    let deadline = Instant::now() + Duration::from_secs(HEAD_SYNC_TIMEOUT_SECS);

    while Instant::now() < deadline {
        let source_before_fork_choice = context.load_live_helper_fork_choice().await;
        let client_fork_choice = load_fork_choice_response(&context.client_under_test).await;
        if client_fork_choice.head == source_before_fork_choice.head {
            return (source_before_fork_choice, client_fork_choice);
        }
        let source_after_fork_choice = context.load_live_helper_fork_choice().await;
        last_observation = Some(capture_head_sync_observation(
            &source_before_fork_choice,
            &client_fork_choice,
            &source_after_fork_choice,
        ));
        if client_fork_choice.head == source_after_fork_choice.head {
            return (source_after_fork_choice, client_fork_choice);
        }

        sleep(Duration::from_secs(1)).await;
    }

    let observation = last_observation
        .expect("sync head wait should record at least one helper/client forkchoice observation");
    panic!(
        "Client under test never matched the local LeanSpec helper head within {} seconds (helper-before head: {:#x} at slot {} [justified={}, finalized={}], client head: {:#x} at slot {} [justified={}, finalized={}], helper-after head: {:#x} at slot {} [justified={}, finalized={}])",
        HEAD_SYNC_TIMEOUT_SECS,
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
                "Starts a local LeanSpec helper, checkpoint-syncs the client under test from a finalized checkpoint, connects it to the helper mesh, and checks that the client catches up to the helper's live head.".to_string(),
                false,
                PostGenesisSyncTestData {
                    client_under_test: client.clone(),
                    genesis_time: checkpoint_sync_genesis_time,
                    wait_for_client_justified_checkpoint: false,
                    use_checkpoint_sync: true,
                    connect_client_to_lean_spec_mesh: true,
                    client_role: ClientUnderTestRole::Validator,
                    source_helper_validator_indices: None,
                    helper_peer_count: SYNC_HELPER_PEER_COUNT,
                    helper_fork_digest_profile,
                },
                test_checkpoint_sync_fresh_start,
            )
            .await;
        }
    }
}

dyn_async! {
    async fn test_checkpoint_sync_fresh_start<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let client_name = test_data.client_under_test.name.clone();

        timeout(
            Duration::from_secs(CHECKPOINT_SYNC_FRESH_START_TIMEOUT_SECS),
            async {
                let mut context = start_post_genesis_sync_context(test, &test_data).await;
                assert!(
                    context.source_fork_choice.finalized.slot > 0,
                    "helper source should expose a non-genesis finalized checkpoint before checkpoint-syncing the client under test"
                );

                let checkpoint_sync_boundary = context.source_fork_choice.finalized.slot;
                let (source_fork_choice, client_fork_choice) =
                    wait_for_client_to_reach_source_head(&mut context).await;
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
                assert_eq!(
                    client_fork_choice.head, source_fork_choice.head,
                    "checkpoint-synced client should eventually report the same live forkchoice head as the helper mesh"
                );
                assert_eq!(
                    client_head_slot, source_head_slot,
                    "matching forkchoice heads should resolve to the same head slot"
                );
            },
        )
        .await
        .unwrap_or_else(|_| {
            panic!(
                "checkpoint-sync fresh-start test for {} exceeded {} seconds",
                client_name, CHECKPOINT_SYNC_FRESH_START_TIMEOUT_SECS
            )
        });
    }
}
