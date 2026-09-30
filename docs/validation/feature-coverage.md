# Pinned Rust feature coverage

This maps every bullet in the pinned Rust README's “Core behavior and boundaries”
section to the Go implementation and contract tests. Native acceptance is tracked
separately; this checklist does not equate mocked lifecycle tests with guest boot.

| Contract | Implementation | Named verification |
|---|---|---|
| Environment membership, six-hour default, permanent/long lifetimes and destructive expiry | `internal/ctl/environment.go`, `lifecycle.go`; CLI | `TestLifetimeDefaultsLongTermsAndPermanentEnvironments`, `TestPartialDeletePreservesFailedVMAndExpiryRetries`, `TestStaleExpiryDoesNotDeleteRecreatedPermanentEnvironment`, `TestExpiryLabelsPreserveDeletionSemantics` |
| Stop/start retains disks, loses memory; deletion removes owned runtime state | agent runtime and engine implementations | `TestStopStartUsesRetainedDiskForColdBootAndResumeForPause`, `TestFailedDeleteKeepsRecordAndReservationsThenRetrySucceeds` |
| Creation persists before execution; incomplete/unknown outcomes remain inspectable | controller environment state and agent runtime | `TestCreationPersistsCompletePlanBeforeAgentContactAndReconcilesLostReply`, `TestFailedCreateRetainsIntentAndResources`, `TestEnvironmentCreationSurvivesClientDisconnect` |
| Zvol preference, configured reservations, host/VM limits | controller scheduler, model constants/validation | `TestVMStoragePrefersUsableZFSThenFallsBackToFiles`, `TestScheduleEnvironmentUsesShadowResourcesAndStableTieOrder`, wire fixture/validation cases |
| Shared scoped capabilities and dashboard controls | capability package, controller dashboard, CLI | `TestExperimentalFreeBSDOptionsKeepTheirEngineLimits`, `TestRustCLIOutputAndRequests`, `TestDashboardMatchesPinnedRustAsset` |
| Initial QEMU/Firecracker SSH, optional sudo, eligible offline resize and container workload defaults | sshboot, agent resource lifecycle, engines | `TestQemuManagedSSHSeedUsesSerdeQuoting`, sshboot/guestconfig Rust fixtures, `TestResourceUpdateRetriesAfterReopenAndKeepsDiskAndIdentity`, `TestContainerStartUnpausesAndChecksReadiness` |
| Persisted Docker/Podman binding | agent bindings and Docker engine | `TestContainerRuntimeBindingSurvivesReopenAndRejectsSwitching`, `TestBoundRuntimeNeverDeletesFromAnotherContainerStore` |
| Firecracker jailer, CPU/memory limits, read-only guest config and shutdown escalation | Firecracker/sandbox engine, guestconfig | `TestLinuxEngineRustGoldenCommandsAndConfiguration`, `TestConfigurationAttachesOneReadOnlyDiskForConfigOrSSH`, `TestSandboxPreparationRecordsOwnershipAndRetainsDisk` |
| Linux egress restrictions and optional guest isolation; unsupported Docker options refused | netcfg, capability validation and agent network restoration | Rust network golden fixtures, capability/agent request tests, `TestConfirmedStopClearsOnlyObsoleteNetworkRecoveryErrors` |
| Host-local image preparation and explicit unsupported fleet features | image builder, storage and CLI | `TestVerifiedRecipesSelectManifestAlgorithm`, `TestQemuRecipeConversionAndFailedPublication`, `TestDockerRecipeUsesSelectedRuntimeAndAlias` |
| Opt-in, stopped-only zvol/strict-reflink backup; atomic refresh and asynchronous cleanup | agent/ctl backup protocol and storage | `TestFailedRefreshPreservesCurrentAndFailedCleanupKeepsReservations`, `TestRestoreIntentSurvivesReopenAndCompletedRetryPreservesNewObservations`, `TestForwardedRestoreLostReplyReplayAndHeadersAreSafe`, `TestNativeZvolBackupRestoreAndOwnership` |

Wire/SQLite compatibility, authentication, routing, command plans, native service
interchange and CLI templates are documented in the phase-specific validation
records. Native engine/storage/SSH/recovery combinations remain subject to the
phase-7 matrix in [roadmap.md](../roadmap.md).
