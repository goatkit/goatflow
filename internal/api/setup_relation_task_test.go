package api

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// TestSetupAssistant_AssignAgentToGroup verifies an existing agent gains rw
// group membership, is idempotent per (agent, team), and validates inputs.
func TestSetupAssistant_AssignAgentToGroup(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	agentLogin := "rel_agent" + sfx
	cleanupAgentByLogin(t, agentLogin)
	cleanupGroupByNameAtEnd(t, "RelTeam"+sfx)
	agentID, err := svc.CreateAgent(ctx, agentLogin, "Rel", "Agent", "", nil, 1)
	require.NoError(t, err)

	teamID, err := svc.CreateGroup(ctx, "RelTeam"+sfx, "relation-task test", 1)
	require.NoError(t, err)

	// Happy path: assign the agent to the team.
	require.NoError(t, svc.AssignAgentToGroups(ctx, agentID, []int{teamID}, 1))

	// Membership row exists with rw permission.
	db, err := database.GetDB()
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM group_user WHERE user_id = ? AND group_id = ? AND permission_key = 'rw'"),
		agentID, teamID).Scan(&n))
	assert.Equal(t, 1, n)

	// Idempotent: re-assigning does not create a duplicate row.
	require.NoError(t, svc.AssignAgentToGroups(ctx, agentID, []int{teamID}, 1))
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM group_user WHERE user_id = ? AND group_id = ?"),
		agentID, teamID).Scan(&n))
	assert.Equal(t, 1, n)

	// Validation: unknown agent.
	err = svc.AssignAgentToGroups(ctx, 9999999, []int{teamID}, 1)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not found")

	// Validation: no teams.
	err = svc.AssignAgentToGroups(ctx, agentID, nil, 1)
	require.Error(t, err)
}

// TestSetupAssistant_AssignQueueToGroup verifies the queue's one owning team
// (queue.group_id) is replaced, the change is idempotent, and inputs are
// validated.
func TestSetupAssistant_AssignQueueToGroup(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	cleanupGroupByNameAtEnd(t, "RelQTeam"+sfx)
	cleanupGroupByNameAtEnd(t, "RelQOther"+sfx)
	cleanupQueueByNameAtEnd(t, "RelQueue"+sfx)
	teamID, err := svc.CreateGroup(ctx, "RelQTeam"+sfx, "relation-task test", 1)
	require.NoError(t, err)
	queueID, err := svc.CreateQueue(ctx, "RelQueue"+sfx, teamID, "relation-task test", 1)
	require.NoError(t, err)

	otherTeamID, err := svc.CreateGroup(ctx, "RelQOther"+sfx, "relation-task test", 1)
	require.NoError(t, err)

	db, err := database.GetDB()
	require.NoError(t, err)
	owner := func() int {
		var gid int
		require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
			"SELECT group_id FROM queue WHERE id = ?"), queueID).Scan(&gid))
		return gid
	}
	require.Equal(t, teamID, owner())

	require.NoError(t, svc.AssignQueueToGroup(ctx, queueID, otherTeamID, 1))
	assert.Equal(t, otherTeamID, owner())

	// Idempotent.
	require.NoError(t, svc.AssignQueueToGroup(ctx, queueID, otherTeamID, 1))
	assert.Equal(t, otherTeamID, owner())

	// Validation: unknown queue.
	err = svc.AssignQueueToGroup(ctx, 9999999, teamID, 1)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not found")

	// Validation: unknown team leaves the owner unchanged.
	err = svc.AssignQueueToGroup(ctx, queueID, 9999999, 1)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not found")
	assert.Equal(t, otherTeamID, owner())

	// Validation: no team.
	err = svc.AssignQueueToGroup(ctx, queueID, 0, 1)
	require.Error(t, err)
}
