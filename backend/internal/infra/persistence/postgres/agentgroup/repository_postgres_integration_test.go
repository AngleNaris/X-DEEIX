package agentgroup

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domainagentgroup "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/agentgroup"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	models "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	conversationrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresAgentGroupAdmissionCreatesOneActiveRun(t *testing.T) {
	db := openAgentGroupPostgresIntegrationDB(t)
	group := seedPostgresAgentGroup(t, db, "admission")
	repo := NewRepo(db)

	const concurrency = 32
	start := make(chan struct{})
	results := make(chan error, concurrency)
	var created atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			run := &domainagentgroup.Run{
				PublicID: fmt.Sprintf("run_admission_%d", index), ClientRunID: fmt.Sprintf("client_admission_%d", index),
				UserID: group.UserID, ConversationID: 101, GroupID: group.ID,
				ConfigSnapshotJSON: "{}", Status: domainagentgroup.RunStatusPending, StartedAt: time.Now(),
			}
			ok, err := repo.CreateAgentGroupRunIfIdle(context.Background(), run)
			if ok {
				created.Add(1)
			}
			results <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent admission error: %v", err)
		}
	}
	if created.Load() != 1 {
		t.Fatalf("created runs = %d, want 1", created.Load())
	}
	var count int64
	if err := db.Model(&models.AgentGroupRun{}).Where("conversation_id = ?", 101).Count(&count).Error; err != nil {
		t.Fatalf("count active runs: %v", err)
	}
	if count != 1 {
		t.Fatalf("stored runs = %d, want 1", count)
	}
}

func TestPostgresAgentGroupRetryRollbackRejectsCrossRunStep(t *testing.T) {
	db := openAgentGroupPostgresIntegrationDB(t)
	group := seedPostgresAgentGroup(t, db, "retry")
	runA := seedPostgresAgentGroupRun(t, db, group, "retry_a", domainagentgroup.RunStatusPausedRetryable)
	runB := seedPostgresAgentGroupRun(t, db, group, "retry_b", domainagentgroup.RunStatusPausedRetryable)
	stepB := seedPostgresAgentGroupStep(t, db, runB.ID, "retry_b", domainagentgroup.StepStatusInterrupted)

	attempt := &domainagentgroup.Attempt{
		PublicID: "attempt_cross_run", StepID: stepB.ID, AttemptNo: 1,
		RetryRequestID: "retry_cross_run", Status: domainagentgroup.AttemptStatusRunning, StartedAt: time.Now(),
	}
	ok, err := NewRepo(db).BeginAgentGroupStepRetry(t.Context(), runA.ID, runA.StateVersion, stepB.ID, attempt)
	if err == nil || ok {
		t.Fatalf("cross-run retry = ok %v err %v, want rollback", ok, err)
	}
	var storedRun models.AgentGroupRun
	if err := db.First(&storedRun, runA.ID).Error; err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if storedRun.Status != domainagentgroup.RunStatusPausedRetryable || storedRun.StateVersion != runA.StateVersion {
		t.Fatalf("run changed after rollback: status=%q version=%d", storedRun.Status, storedRun.StateVersion)
	}
	var attemptCount int64
	if err := db.Model(&models.AgentGroupStepAttempt{}).Where("step_id = ?", stepB.ID).Count(&attemptCount).Error; err != nil {
		t.Fatalf("count attempts: %v", err)
	}
	if attemptCount != 0 {
		t.Fatalf("cross-run retry created %d attempts, want 0", attemptCount)
	}
}

func TestPostgresAgentGroupLeaseRecoveryRunsOnce(t *testing.T) {
	db := openAgentGroupPostgresIntegrationDB(t)
	group := seedPostgresAgentGroup(t, db, "lease")
	run := seedPostgresAgentGroupRun(t, db, group, "lease", domainagentgroup.RunStatusRunning)
	step := seedPostgresAgentGroupStep(t, db, run.ID, "lease", domainagentgroup.StepStatusRunning)
	expired := time.Now().Add(-time.Hour)
	attempt := models.AgentGroupStepAttempt{
		PublicID: "attempt_lease", StepID: step.ID, AttemptNo: 1, Status: domainagentgroup.AttemptStatusRunning,
		LeaseExpiresAt: &expired, StartedAt: expired,
	}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatalf("create attempt: %v", err)
	}

	const concurrency = 16
	start := make(chan struct{})
	results := make(chan error, concurrency)
	var recovered atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			count, err := NewRepo(db).RecoverExpiredAttemptLeases(context.Background(), time.Now())
			recovered.Add(count)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent lease recovery error: %v", err)
		}
	}
	if recovered.Load() != 1 {
		t.Fatalf("recovered runs = %d, want 1", recovered.Load())
	}
	if err := db.First(&attempt, attempt.ID).Error; err != nil {
		t.Fatalf("reload attempt: %v", err)
	}
	if attempt.Status != domainagentgroup.AttemptStatusInterrupted {
		t.Fatalf("attempt status = %q, want interrupted", attempt.Status)
	}
	if err := db.First(&run, run.ID).Error; err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if run.Status != domainagentgroup.RunStatusPausedRetryable || run.RetryableStepID == nil || *run.RetryableStepID != step.ID {
		t.Fatalf("recovered run = %#v", run)
	}
}

func TestPostgresAgentGroupDeleteRacesLeaveNoOrphans(t *testing.T) {
	db := openAgentGroupPostgresIntegrationDB(t)
	t.Run("run", func(t *testing.T) {
		for i := 0; i < 24; i++ {
			group := seedPostgresAgentGroup(t, db, fmt.Sprintf("delete_run_%d", i))
			run := &domainagentgroup.Run{
				PublicID: fmt.Sprintf("run_delete_%d", i), ClientRunID: fmt.Sprintf("client_delete_%d", i),
				UserID: group.UserID, ConversationID: uint(1000 + i), GroupID: group.ID,
				ConfigSnapshotJSON: "{}", Status: domainagentgroup.RunStatusPending, StartedAt: time.Now(),
			}
			start := make(chan struct{})
			var deleteErr, createErr error
			var created bool
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				deleteErr = NewRepo(db).DeleteAgentGroupByPublicID(context.Background(), group.UserID, group.PublicID)
			}()
			go func() {
				defer wg.Done()
				<-start
				created, createErr = NewRepo(db).CreateAgentGroupRunIfIdle(context.Background(), run)
			}()
			close(start)
			wg.Wait()
			assertPostgresReferenceRaceOutcome(t, db, group.ID, created, createErr, deleteErr, &models.AgentGroupRun{})
		}
	})
	t.Run("conversation", func(t *testing.T) {
		for i := 0; i < 24; i++ {
			group := seedPostgresAgentGroup(t, db, fmt.Sprintf("delete_conversation_%d", i))
			conversation := &domainconversation.Conversation{
				UserID: group.UserID, AgentGroupID: &group.ID, PublicID: fmt.Sprintf("conversation_delete_%d", i),
				Title: "race", LabelsJSON: "[]", SessionKey: fmt.Sprintf("session_delete_%d", i), Status: "active",
			}
			start := make(chan struct{})
			var deleteErr, createErr error
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				deleteErr = NewRepo(db).DeleteAgentGroupByPublicID(context.Background(), group.UserID, group.PublicID)
			}()
			go func() {
				defer wg.Done()
				<-start
				createErr = conversationrepo.NewRepo(db).CreateConversation(context.Background(), conversation)
			}()
			close(start)
			wg.Wait()
			assertPostgresReferenceRaceOutcome(t, db, group.ID, createErr == nil, createErr, deleteErr, &models.Conversation{})
		}
	})
}

func TestPostgresAgentGroupDeletePreservesConversationAndRunHistory(t *testing.T) {
	db := openAgentGroupPostgresIntegrationDB(t)
	group := seedPostgresAgentGroup(t, db, "delete_history")
	conversation := models.Conversation{
		UserID: group.UserID, AgentGroupID: &group.ID, PublicID: "conversation_delete_history",
		Title: "history", LabelsJSON: "[]", SessionKey: "session_delete_history", Status: "active",
	}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	run := seedPostgresAgentGroupRun(t, db, group, "delete_history", domainagentgroup.RunStatusCompleted)

	if err := NewRepo(db).DeleteAgentGroupByPublicID(context.Background(), group.UserID, group.PublicID); err != nil {
		t.Fatalf("delete group with history: %v", err)
	}

	var storedConversation models.Conversation
	if err := db.First(&storedConversation, conversation.ID).Error; err != nil {
		t.Fatalf("load preserved conversation: %v", err)
	}
	if storedConversation.AgentGroupID != nil {
		t.Fatalf("conversation remains bound to deleted group: %#v", storedConversation.AgentGroupID)
	}
	var storedRun models.AgentGroupRun
	if err := db.First(&storedRun, run.ID).Error; err != nil {
		t.Fatalf("load preserved run: %v", err)
	}
	if storedRun.GroupID != group.ID {
		t.Fatalf("run group id changed: %d", storedRun.GroupID)
	}
	groups, err := NewRepo(db).ListAgentGroups(context.Background(), group.UserID, 0)
	if err != nil {
		t.Fatalf("list groups after delete: %v", err)
	}
	if len(groups) != 0 {
		t.Fatalf("deleted group still listed: %#v", groups)
	}
}

func TestPostgresAgentGroupMemberUpdatePersistsReasoningEffort(t *testing.T) {
	db := openAgentGroupPostgresIntegrationDB(t)
	group := seedPostgresAgentGroup(t, db, "member_reasoning")
	role := models.ConversationRole{UserID: group.UserID, PublicID: "role_member_reasoning", Name: "worker", Status: "active"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	member := models.AgentGroupMember{
		PublicID: "member_reasoning", GroupID: group.ID, RoleID: role.ID,
		MemberType: domainagentgroup.MemberTypeWorker, Enabled: true,
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	effort := "high"
	if _, err := NewRepo(db).UpdateAgentGroupMemberByPublicID(context.Background(), group.ID, group.UserID, member.PublicID, domainagentgroup.MemberPatch{ReasoningEffort: &effort}); err != nil {
		t.Fatalf("update member reasoning effort: %v", err)
	}
	var stored models.AgentGroupMember
	if err := db.First(&stored, member.ID).Error; err != nil {
		t.Fatalf("reload member: %v", err)
	}
	if stored.ReasoningEffort != effort {
		t.Fatalf("reasoning effort = %q, want %q", stored.ReasoningEffort, effort)
	}
}

func assertPostgresReferenceRaceOutcome(t *testing.T, db *gorm.DB, groupID uint, created bool, createErr error, deleteErr error, referenceModel interface{}) {
	t.Helper()
	if deleteErr != nil {
		t.Fatalf("delete error = %v", deleteErr)
	}
	if created {
		if createErr != nil {
			t.Fatalf("create reported success with error: %v", createErr)
		}
	} else if createErr != nil && !errors.Is(createErr, repository.ErrNotFound) {
		t.Fatalf("unexpected create error when delete won: %v", createErr)
	}
	var group models.AgentGroup
	if err := db.Unscoped().First(&group, groupID).Error; err != nil {
		t.Fatalf("load deleted group: %v", err)
	}
	if !group.DeletedAt.Valid {
		t.Fatalf("group %d was not soft deleted", groupID)
	}

	var referenceCount int64
	column := "group_id"
	if _, ok := referenceModel.(*models.Conversation); ok {
		column = "agent_group_id"
	}
	if err := db.Model(referenceModel).Where(column+" = ?", groupID).Count(&referenceCount).Error; err != nil {
		t.Fatalf("count references: %v", err)
	}
	if _, ok := referenceModel.(*models.Conversation); ok {
		if referenceCount != 0 {
			t.Fatalf("deleted group still owns %d conversations", referenceCount)
		}
		return
	}
	expected := int64(0)
	if created {
		expected = 1
	}
	if referenceCount != expected {
		t.Fatalf("run references = %d, want %d", referenceCount, expected)
	}
}

func openAgentGroupPostgresIntegrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("DEEIX_TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("set DEEIX_TEST_DATABASE_DSN to run PostgreSQL Agent Group integration tests")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatalf("resolve postgres admin db: %v", err)
	}
	schemaName := fmt.Sprintf("deeix_test_agent_group_%d", time.Now().UnixNano())
	if err := admin.Exec(`CREATE SCHEMA ` + schemaName).Error; err != nil {
		_ = adminSQL.Close()
		t.Fatalf("create test schema: %v", err)
	}
	db, err := gorm.Open(postgres.Open(postgresDSNWithSearchPath(dsn, schemaName)), &gorm.Config{})
	if err != nil {
		_ = admin.Exec(`DROP SCHEMA IF EXISTS ` + schemaName + ` CASCADE`).Error
		_ = adminSQL.Close()
		t.Fatalf("open isolated postgres schema: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve isolated postgres db: %v", err)
	}
	sqlDB.SetMaxOpenConns(32)
	t.Cleanup(func() {
		_ = sqlDB.Close()
		_ = admin.Exec(`DROP SCHEMA IF EXISTS ` + schemaName + ` CASCADE`).Error
		_ = adminSQL.Close()
	})
	if err := db.AutoMigrate(
		&models.Conversation{},
		&models.ConversationRole{},
		&models.AgentGroup{},
		&models.AgentGroupMember{},
		&models.AgentGroupRun{},
		&models.AgentGroupStep{},
		&models.AgentGroupStepAttempt{},
	); err != nil {
		t.Fatalf("migrate Agent Group tables: %v", err)
	}
	return db
}

func postgresDSNWithSearchPath(dsn string, schemaName string) string {
	parsed, err := url.Parse(dsn)
	if err == nil && (parsed.Scheme == "postgres" || parsed.Scheme == "postgresql") {
		query := parsed.Query()
		query.Set("search_path", schemaName+",public")
		parsed.RawQuery = query.Encode()
		return parsed.String()
	}
	return dsn + " search_path=" + schemaName + ",public"
}

func seedPostgresAgentGroup(t *testing.T, db *gorm.DB, suffix string) models.AgentGroup {
	t.Helper()
	group := models.AgentGroup{UserID: 41, PublicID: "group_" + suffix, Name: suffix, Status: domainagentgroup.GroupStatusActive}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create group %s: %v", suffix, err)
	}
	return group
}

func seedPostgresAgentGroupRun(t *testing.T, db *gorm.DB, group models.AgentGroup, suffix string, status string) models.AgentGroupRun {
	t.Helper()
	run := models.AgentGroupRun{
		PublicID: "run_" + suffix, ClientRunID: "client_" + suffix, UserID: group.UserID,
		ConversationID: group.ID + 1000, GroupID: group.ID, ConfigSnapshotJSON: "{}",
		Status: status, StateVersion: 1, StartedAt: time.Now(),
	}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("create run %s: %v", suffix, err)
	}
	return run
}

func seedPostgresAgentGroupStep(t *testing.T, db *gorm.DB, runID uint, suffix string, status string) models.AgentGroupStep {
	t.Helper()
	step := models.AgentGroupStep{
		PublicID: "step_" + suffix, GroupRunID: runID, Sequence: 1,
		StepType: domainagentgroup.StepTypeMemberExecute, ActorMemberPublicID: "member_" + suffix, Status: status,
	}
	if err := db.Create(&step).Error; err != nil {
		t.Fatalf("create step %s: %v", suffix, err)
	}
	return step
}
