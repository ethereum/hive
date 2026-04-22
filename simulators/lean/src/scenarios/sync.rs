use crate::scenarios::helper::{
    default_genesis_time, fork_choice_head_slot, load_fork_choice_response, run_data_test,
    start_post_genesis_sync_context, ClientUnderTestRole, ForkChoiceResponse,
    HelperGossipForkDigestProfile, PostGenesisSyncContext, PostGenesisSyncTestData,
    SourceCheckpointKind,
};
use crate::{lean_clients, selected_lean_devnet, LeanDevnet};
use hivesim::{dyn_async, Client, Test};
use std::time::Duration;
use tokio::time::{sleep, timeout, Instant};

const HEAD_SYNC_TIMEOUT_SECS: u64 = 300;
const CHECKPOINT_SYNC_FRESH_START_TIMEOUT_SECS: u64 = 480;
const SYNC_HELPER_PEER_COUNT: usize = 3;
const SYNC_HELPER_VALIDATORS_WITH_CLIENT_RESERVED: &str = "1,2";

fn helper_fork_digest_profile_for_client(client_type: &str) -> HelperGossipForkDigestProfile {
    if selected_lean_devnet() == LeanDevnet::Devnet4 {
        return HelperGossipForkDigestProfile::SelectedDevnet;
    }

    if client_type.starts_with("lantern") {
        return HelperGossipForkDigestProfile::SelectedDevnet;
    }

    HelperGossipForkDigestProfile::LegacyDevnet0
}

fn client_role_for_sync(client_type: &str) -> ClientUnderTestRole {
    if client_type.starts_with("lantern") || client_type.starts_with("zeam") {
        return ClientUnderTestRole::Validator;
    }

    ClientUnderTestRole::Observer
}

fn source_helper_validator_indices_for_client(client_type: &str) -> Option<String> {
    if client_type.starts_with("lantern") || client_type.starts_with("zeam") {
        return Some(SYNC_HELPER_VALIDATORS_WITH_CLIENT_RESERVED.to_string());
    }

    None
}

struct HeadSyncObservation {
    source_before_head: String,
    source_before_head_slot: u64,
    source_before_justified_slot: u64,
    source_before_finalized_slot: u64,
    client_head: String,
    client_head_slot: u64,
    client_justified_slot: u64,
    client_finalized_slot: u64,
    source_after_head: String,
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
        source_before_head: format!("{:#x}", source_before.head),
        source_before_head_slot: fork_choice_head_slot(source_before),
        source_before_justified_slot: source_before.justified.slot,
        source_before_finalized_slot: source_before.finalized.slot,
        client_head: format!("{:#x}", client.head),
        client_head_slot: fork_choice_head_slot(client),
        client_justified_slot: client.justified.slot,
        client_finalized_slot: client.finalized.slot,
        source_after_head: format!("{:#x}", source_after.head),
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
        "Client under test never matched the local LeanSpec helper head within {} seconds (helper-before head: {} at slot {} [justified={}, finalized={}], client head: {} at slot {} [justified={}, finalized={}], helper-after head: {} at slot {} [justified={}, finalized={}])",
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
            let client_role = client_role_for_sync(&client.name);

            run_data_test(
                test,
                "sync: checkpoint sync fresh start".to_string(),
                "Starts a local LeanSpec helper, checkpoint-syncs the client under test from a finalized checkpoint, connects it to the helper mesh, and checks that the client catches up to the helper's live head.".to_string(),
                false,
                PostGenesisSyncTestData {
                    client_under_test: client.clone(),
                    genesis_time: checkpoint_sync_genesis_time,
                    source_checkpoint_kind: SourceCheckpointKind::Finalized,
                    wait_for_client_justified_checkpoint: false,
                    use_checkpoint_sync: true,
                    connect_client_to_lean_spec_mesh: true,
                    client_role,
                    source_helper_validator_indices: source_helper_validator_indices_for_client(
                        &client.name,
                    ),
                    helper_peer_count: SYNC_HELPER_PEER_COUNT,
                    helper_fork_digest_profile: helper_fork_digest_profile_for_client(
                        &client.name,
                    ),
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

dyn_async! {
    async fn test_sync_network_finalized_passed_local_head<'a>() {
        //client is started and made to run alongside leanspec client from genesis, then paused in some manner to simulate bad network conditions etc, then
        //unpaused once the network finalized slot has passed the clients local head slot to see if the client properly starts resyncing until it catches back up to network head
    }
}

dyn_async! {
    async fn test_sync_bad_outlier<'a>() {
        //client is started and made to run alongside leanspec client from genesis, then paused in some manner to simulate bad network conditions etc, then
        //unpaused once the network finalized slot has passed the clients local head slot to see if the client properly starts resyncing until it catches back up to network head
        //The difference is that there also exists two other clients/helper that is connected to the client being tested that are either behind the network head or way ahead of the network head,
        //this effect can be achieved artificially using mock clients if needed
    }
}
